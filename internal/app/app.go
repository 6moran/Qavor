package app

import (
	"strconv"

	agentpkg "Qavor/internal/agent"
	"Qavor/internal/agent/localfs"
	"Qavor/internal/agent/localfs/security"
	"Qavor/internal/api"
	agentctrl "Qavor/internal/api/v1/agent"
	chatctrl "Qavor/internal/api/v1/chat"
	evaluationctrl "Qavor/internal/api/v1/evaluation"
	mcpserverctrl "Qavor/internal/api/v1/mcp_server"
	mindmapctrl "Qavor/internal/api/v1/mindmap"
	ocrctrl "Qavor/internal/api/v1/ocr"
	ragctrl "Qavor/internal/api/v1/rag"
	systemctrl "Qavor/internal/api/v1/system"
	workspaceapi "Qavor/internal/api/v1/workspace"
	contextmgr "Qavor/internal/context"
	"Qavor/internal/embedding"
	"Qavor/internal/eventbus"
	"Qavor/internal/ingestion"
	"Qavor/internal/mcp"
	longterm "Qavor/internal/memory/long_term"
	shortterm "Qavor/internal/memory/short_term"
	"Qavor/internal/model/entity"
	documentqueue "Qavor/internal/queue"
	"Qavor/internal/rag"
	"Qavor/internal/repository"
	"Qavor/internal/run"
	"Qavor/internal/service"
	"Qavor/internal/skill"
	skillapi "Qavor/internal/skill/api"
	"Qavor/internal/skill/remote"
	"Qavor/internal/sse"

	"github.com/cloudwego/eino/adk"

	tracectrl "Qavor/internal/api/v1/trace"
	"Qavor/internal/llm"
	"Qavor/internal/store"
	"Qavor/internal/tool"
	"Qavor/internal/tool/builtin"
	"Qavor/internal/tool/builtin/websearch"
	"Qavor/internal/trace"
	"Qavor/internal/worker"
	"Qavor/pkg/config"
	"Qavor/pkg/database"
	"Qavor/pkg/logger"
	"Qavor/pkg/minio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cloudwego/eino/adk/backgroundtask"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// imageUploader 将 ObjectStorage 适配为 ingestion.ImageUploader。
type imageUploader struct {
	storage service.ObjectStorage
}

// UploadImage 上传图片字节到对象存储，返回可公开访问的 URL。
func (u imageUploader) UploadImage(folder, filename string, data []byte) (string, error) {
	// 优先按扩展名映射类型（http.DetectContentType 无法识别 TIFF 等格式）
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	obj, err := u.storage.UploadReader(folder, filename, contentType, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("上传图片 %s/%s 失败: %w", folder, filename, err)
	}
	return obj.URL, nil
}

// qavorDataDir 返回工作目录下的 qavor 数据目录，并确保目录存在。
// 工作目录无法解析或创建失败时返回 error，调用方应终止初始化。
func qavorDataDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("无法解析工作目录: %w", err)
	}
	dir := filepath.Join(cwd, "qavor")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建 qavor 数据目录失败: %w", err)
	}
	return dir, nil
}

// defaultSkillsDir 返回工作目录下 qavor/skills 目录，并确保目录存在。
// 目录创建失败时返回 error，调用方应终止初始化。
func defaultSkillsDir() (string, error) {
	dataDir, err := qavorDataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(dataDir, "skills")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建 Skill 目录失败: %w", err)
	}
	return dir, nil
}

// expandSkillDir 展开路径开头的 ~ 或 ~/ 为当前用户主目录。
// 其余形式（绝对路径、相对路径、~user/）原样返回。
func expandSkillDir(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path[1:], "/")), nil
	}
	return path, nil
}

// resolveWorkspaceRoot 将 agent 工作区根目录解析为绝对路径。
// 相对 workspace_root（默认 data/workspaces）依赖进程 cwd 解析；
// 若从某 agent 工作区内（data/workspaces/<slug>）启动，相对路径会自我嵌套，
// 生成 data/workspaces/<slug>/data/workspaces/<slug>/... 的重复目录。这里统一转绝对路径，
// 并检测 cwd 已位于 agent 工作区内时直接报错，避免静默污染磁盘。
func resolveWorkspaceRoot(root string) (string, error) {
	if root == "" {
		root = "data/workspaces"
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("解析 workspace_root(%q) 失败: %w", root, err)
	}
	if err := checkCwdNotInAgentWorkspace(); err != nil {
		return "", err
	}
	return abs, nil
}

// checkCwdNotInAgentWorkspace 检查进程 cwd 是否位于某个 agent 工作区内。
// 所有 agent 工作区目录均以 agent-<纳秒时间戳> 命名（agent_service.generateSlug）；
// 若 cwd 路径含该特征段，说明从工作区内启动，相对 workspace_root 会自我嵌套，拒绝启动。
func checkCwdNotInAgentWorkspace() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %w", err)
	}
	for seg := range strings.SplitSeq(filepath.Clean(cwd), string(filepath.Separator)) {
		if strings.HasPrefix(seg, "agent-") {
			return fmt.Errorf("检测到进程工作目录位于 agent 工作区内(%s)：相对 workspace_root 会自我嵌套，请从项目根目录启动服务", cwd)
		}
	}
	return nil
}

// App 应用结构体
type App struct {
	cfg              *config.Config
	postgresDB       *gorm.DB
	redis            *redis.Client
	router           *api.Router
	server           *http.Server
	workerStop       context.CancelFunc
	workerDone       chan struct{}
	runWorkerStop    context.CancelFunc
	traceJanitorStop context.CancelFunc
	evaluationStop   context.CancelFunc
	traceWriter      *trace.Writer
	mcpManager       *mcp.MCPManager
	bgManager        *backgroundtask.Manager
	evaluationSvc    service.EvaluationService
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// Initialize 初始化应用
func (a *App) Initialize() error {
	// 1. 加载配置
	if err := a.initConfig(); err != nil {
		return err
	}

	// 2. 初始化日志
	if err := a.initLogger(); err != nil {
		return err
	}

	// 3. 初始化数据库
	if err := a.initDatabase(); err != nil {
		return err
	}

	// 4. 初始化 MinIO（可选）
	if err := a.initMinIO(); err != nil {
		logger.Warn("MinIO 初始化失败，文件上传功能将不可用", zap.Error(err))
	}

	// 5. 初始化依赖
	if err := a.initDependencies(); err != nil {
		return err
	}

	// 6. 初始化路由
	a.initRouter()

	// 7. 初始化服务器
	a.initServer()

	return nil
}

// initConfig 加载配置
func (a *App) initConfig() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	if err := cfg.ValidateAuth(); err != nil {
		return fmt.Errorf("认证配置无效: %w", err)
	}
	// 归一化 workspace_root 为绝对路径：相对路径依赖启动 cwd，
	// 若从某 agent 工作区内启动会自我嵌套（见 resolveWorkspaceRoot 注释）。
	root, err := resolveWorkspaceRoot(cfg.Agent.WorkspaceRoot)
	if err != nil {
		return err
	}
	cfg.Agent.WorkspaceRoot = root
	a.cfg = cfg
	return nil
}

// initLogger 初始化日志
func (a *App) initLogger() error {
	if err := logger.Init(&a.cfg.Log); err != nil {
		return fmt.Errorf("日志初始化失败: %w", err)
	}

	// 打印启动横幅
	logger.Info("=========================================")
	logger.Info(fmt.Sprintf("欢迎使用 %s", a.cfg.App.Name))
	logger.Info(fmt.Sprintf("版本: %s", a.cfg.App.Version))
	logger.Info(fmt.Sprintf("模式: %s", a.cfg.App.Mode))
	logger.Info("配置加载成功")
	logger.Info("=========================================")

	return nil
}

// initDatabase 初始化数据库
func (a *App) initDatabase() error {
	// 初始化 PostgreSQL
	postgresDB, err := database.InitPostgres(&a.cfg.Database.Postgres)
	if err != nil {
		return fmt.Errorf("PostgreSQL 初始化失败: %w", err)
	}
	a.postgresDB = postgresDB

	// 根据配置决定是否自动迁移数据库表
	if a.cfg.Database.AutoMigrate {
		logger.Info("开始数据库迁移...")
		if err := a.postgresDB.AutoMigrate(
			&entity.Agent{},
			&entity.Conversation{},
			&entity.Message{},
			&entity.ToolCall{},
			&entity.AgentRun{},
			&entity.SubagentThread{},
			&entity.Model{},
			&entity.SystemSetting{},
			&entity.Skill{},
			&entity.KnowledgeBase{},
			&entity.KnowledgeFile{},
			&entity.KnowledgeChunk{},
			&entity.DocumentProcessingJob{},
			&entity.AgentTrace{},
			&entity.AgentTraceSpan{},
			&entity.TraceRecord{},
			&entity.TraceSpan{},
			&entity.LongTermMemory{},
			&entity.EvaluationDataset{},
			&entity.EvaluationDatasetItem{},
			&entity.EvaluationRun{},
			&entity.EvaluationRunResult{},
		); err != nil {
			logger.Warn("数据库迁移警告", zap.Error(err))
		} else {
			logger.Info("数据库迁移完成")
		}
	} else {
		logger.Info("数据库自动迁移已禁用，跳过迁移步骤")
	}

	// 初始化 Redis（可选）
	rs, err := database.InitRedis(&a.cfg.Database.Redis)
	if err != nil {
		logger.Warn("Redis 初始化失败，将不影响核心功能", zap.Error(err))
	}
	a.redis = rs

	return nil
}

// initMinIO 初始化 MinIO 存储
func (a *App) initMinIO() error {
	if err := minio.Init(&a.cfg.Database.MinIO); err != nil {
		return fmt.Errorf("MinIO 初始化失败: %w", err)
	}
	return nil
}

// initDependencies 初始化依赖注入
func (a *App) initDependencies() error {
	// 创建 Repository
	knowledgeBaseRepo := repository.NewKnowledgeBaseRepository(a.postgresDB)
	knowledgeFileRepo := repository.NewKnowledgeFileRepository(a.postgresDB)
	knowledgeChunkRepo := repository.NewKnowledgeChunkRepository(a.postgresDB)
	processingJobRepo := repository.NewDocumentProcessingJobRepository(a.postgresDB)
	modelRepo := repository.NewModelRepository(a.postgresDB)
	systemSettingRepo := repository.NewSystemSettingRepository(a.postgresDB)
	conversationRepo := repository.NewConversationRepository(a.postgresDB)
	messageRepo := repository.NewMessageRepository(a.postgresDB)
	agentRepo := repository.NewAgentRepository(a.postgresDB)

	var queue documentqueue.DocumentQueue
	// 由于需要依赖redis,所以没写在pkg
	if a.redis != nil {
		candidate, err := documentqueue.NewRedisDocumentQueue(
			a.redis,
			a.cfg.DocumentQueue.ParseStream,
			a.cfg.DocumentQueue.ParseGroup,
			a.cfg.DocumentQueue.MaxStreamLength,
		)
		if err == nil {
			queueCtx, cancelQueue := context.WithTimeout(context.Background(), 5*time.Second)
			err = candidate.EnsureGroup(queueCtx)
			cancelQueue()
		}
		if err == nil {
			queue = candidate
		} else {
			logger.Warn("文档处理队列初始化失败，文档异步处理将不可用", zap.Error(err))
		}
	}

	// 创建 Service
	authSvc := service.NewAuthService(a.cfg.Auth)
	modelSvc := service.NewModelService(modelRepo)
	ragSettingsSvc := service.NewRAGSettingsService(systemSettingRepo, modelRepo)
	systemConfigSvc := service.NewSystemConfigService(systemSettingRepo, modelRepo)
	systemCtrl := systemctrl.NewController(ragSettingsSvc, systemConfigSvc)
	ocrCtrl := ocrctrl.NewController(systemConfigSvc)
	storage := service.NewMinIOObjectStorage()
	knowledgeBaseSvc := service.NewKnowledgeBaseService(knowledgeBaseRepo, modelRepo, knowledgeFileRepo, storage, agentRepo)
	knowledgeFileSvc := service.NewKnowledgeFileService(knowledgeBaseRepo, knowledgeFileRepo, processingJobRepo, storage, queue, knowledgeChunkRepo)
	processingJobSvc := service.NewProcessingJobService(processingJobRepo, knowledgeFileRepo, queue)
	agentSvc := service.NewAgentService(agentRepo, a.cfg.Agent.WorkspaceRoot)
	mindmapSvc := service.NewMindmapService(knowledgeBaseRepo, knowledgeFileRepo, knowledgeChunkRepo, modelSvc)
	mindmapCtrl := mindmapctrl.NewController(mindmapSvc)

	// —— 链路追踪 Tracer 装配（提前到 RAG 组装之前创建，供 Runtime/Handler/RAG/Worker 共用）——
	var tracer *trace.Tracer
	var traceSpanRepo trace.TraceRepository
	if a.cfg.Trace.Enabled && a.postgresDB != nil {
		traceSpanRepo = repository.NewTraceSpanRepository(a.postgresDB)
		traceWriter := trace.NewWriter(traceSpanRepo, trace.WriterConfig{
			BufferSize: a.cfg.Trace.WriterBufferSize,
		})
		a.traceWriter = traceWriter
		tracer = trace.NewTracer(traceWriter, trace.Config{
			Enabled:          a.cfg.Trace.Enabled,
			ContentMode:      a.cfg.Trace.ContentMode,
			MaxContentLength: a.cfg.Trace.MaxContentLength,
			Retention:        time.Duration(a.cfg.Trace.RetentionDays) * 24 * time.Hour,
			TracedRoutes:     a.cfg.Trace.TracedRoutes,
		})
		callbacks.AppendGlobalHandlers(trace.NewHandler(tracer))
		logger.Info("链路追踪 Tracer 已装配")
	}

	// 构造按知识库绑定模型解析的 RAG 依赖,由于需要依赖其他模块,所以在这里初始化
	// 模型连接信息来自模型管理表；RAG 配置文件只提供分块、TopK、超时等算法默认值。
	var (
		indexer rag.DocumentIndexer = rag.NewDynamicDocumentIndexer(
			knowledgeBaseRepo,
			modelSvc,
			knowledgeChunkRepo,
			a.cfg.RAG.ChunkTokens,
			a.cfg.RAG.ChunkOverlapTokens,
			a.cfg.RAG.Embedding.BatchSize,
			a.cfg.RAG.Embedding.Dimension,
		)
		// 向量召回按知识库绑定的 Embedding 模型分组并保留独立排名。
		dynamicVectorRetriever = rag.NewDynamicRetriever(
			knowledgeBaseRepo,
			modelSvc,
			knowledgeChunkRepo,
			a.cfg.RAG.VectorTopK,
		)
		keywordRetriever = rag.NewKeywordRetriever(knowledgeChunkRepo, a.cfg.RAG.KeywordTopK)
		dynamicReranker  = rag.NewDynamicReranker(ragSettingsSvc, modelSvc, tracer)
		hybridRetriever  = rag.NewHybridRetriever(
			dynamicVectorRetriever,
			keywordRetriever,
			dynamicReranker,
			rag.HybridConfig{
				VectorTopK: a.cfg.RAG.VectorTopK, KeywordTopK: a.cfg.RAG.KeywordTopK,
				FusedTopK: a.cfg.RAG.FusedTopK, RerankTopK: a.cfg.RAG.RerankTopK, RRFK: a.cfg.RAG.RRFK,
			},
		)
		// 快速回答与独立检索共享同一个混合检索器实例。
		answerer rag.AnswerChain = rag.NewDynamicAnswerEngine(
			knowledgeBaseRepo,
			modelSvc,
			hybridRetriever,
		)
		ragCtrl *ragctrl.Controller
	)
	ragSvc := service.NewRAGService(a.cfg.RAG, knowledgeBaseRepo, hybridRetriever, answerer, tracer)
	ragCtrl = ragctrl.NewController(ragSvc, a.cfg.RAG.RequestTimeoutSeconds)
	// 检索测试与示例问题服务：复用 RAG 检索链路与模型解析能力。
	knowledgeQuerySvc := service.NewKnowledgeQueryService(a.cfg.RAG, knowledgeBaseRepo, knowledgeFileRepo, ragSvc, modelSvc)

	if queue != nil {
		workerCtx, cancelWorker := context.WithCancel(context.Background())
		a.workerStop = cancelWorker
		a.workerDone = make(chan struct{})
		imgUploader := imageUploader{storage: storage}
		ocrEngine, ocrAPIBaseURL, ocrAPIKey, ocrAPIModel := ocrEngineForParser(workerCtx, systemSettingRepo, systemConfigSvc)
		parser := ingestion.NewParser(
			ingestion.NewPythonParser(a.cfg.DocumentParser.PythonPath, "pkg/documentparser/python/parse_document.py", imgUploader).
				WithOCR(ocrEngine, ocrAPIBaseURL, ocrAPIKey, ocrAPIModel),
			imgUploader,
		)
		documentWorker := worker.NewDocumentWorker(queue, processingJobRepo, knowledgeFileRepo, storage, parser, indexer)
		hostname, _ := os.Hostname()
		workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())
		options := worker.DocumentWorkerOptions{
			ReadBlock:        time.Duration(a.cfg.DocumentQueue.ReadBlockSeconds) * time.Second,
			PendingCheck:     time.Duration(a.cfg.DocumentQueue.PendingCheckSeconds) * time.Second,
			PendingMinIdle:   time.Duration(a.cfg.DocumentQueue.PendingMinIdleMinutes) * time.Minute,
			PendingClaimSize: a.cfg.DocumentQueue.PendingClaimCount,
		}
		go func() {
			defer close(a.workerDone)
			documentWorker.Run(workerCtx, workerID, options)
		}()
	} else {
		logger.Warn("Redis 不可用，文档异步处理 Worker 未启动")
	}

	// 初始化 MCPManager
	dataDir, err := qavorDataDir()
	if err != nil {
		return err
	}
	fileStore, err := store.NewMCPServerFileStore(dataDir)
	if err != nil {
		logger.Warn("MCP 配置文件加载失败", zap.Error(err))
		fileStore = store.NewEmptyMCPServerFileStore()
	}
	mcpManager := mcp.NewMCPManager(fileStore)
	a.mcpManager = mcpManager

	// 创建 MCP Server 服务和控制器
	mcpServerSvc := service.NewMCPServerService(fileStore, mcpManager)
	mcpServerCtrl := mcpserverctrl.NewController(mcpServerSvc)

	// 收集预热白名单：从所有 Agent 配置中提取 MCP server name
	whitelist := a.buildMCPWhitelist(agentRepo)
	go mcpManager.Preheat(whitelist)

	// 创建 ToolVectorizer：注入 provider 懒创建工厂。
	// 工厂每次检索时从数据库 system_settings 解析当前配置的向量模型
	// （前端设置页"基本设置 → 向量检索模型"），检测到模型切换会自动清空索引
	// 并释放旧 provider，下次检索懒重建（热重载）。未配置 → 不激活（全量注入）。
	vectorizer := mcp.NewToolVectorizer(mcpManager, func(ctx context.Context) (embedding.Client, string, error) {
		sysCfg, err := systemConfigSvc.Get(ctx)
		if err != nil {
			return nil, "", err
		}
		if sysCfg.MCPRetrievalEmbedModel == "" {
			// 未配置 → 不激活
			return nil, "", nil
		}
		key := "db:" + sysCfg.MCPRetrievalEmbedModel
		id, perr := strconv.ParseUint(sysCfg.MCPRetrievalEmbedModel, 10, 32)
		if perr != nil {
			return nil, key, perr
		}
		embClient, cerr := modelSvc.CreateEmbeddingClient(ctx, uint(id))
		return embClient, key, cerr
	})

	// 热重载：系统配置更新 mcp_retrieval_embed_model 后清空向量索引，
	// 下次 SelectTools 懒重建（provider 工厂会重新解析新模型）。
	systemConfigSvc.SetMCPRetrievalModelChangeCallback(func() {
		vectorizer.Clear()
	})

	// 创建 WebSearch 工具
	// 按 config.App.Mode 与 API Key 决定：真实 Provider / Mock（debug 无 Key）/ 不注册（release 无 Key）
	wsTool, wsErr := websearch.NewTool(a.cfg.WebSearch, a.cfg.App.Mode)
	if wsErr != nil {
		logger.Warn("WebSearch 工具初始化失败，跳过注册", zap.Error(wsErr))
	}

	// 创建 ToolRegistry
	toolRegistry := tool.NewDefaultRegistry()
	toolProvider := builtin.NewBuiltinToolProvider(ragSvc, wsTool)
	toolRegistry.RegisterFromProvider(toolProvider)

	// 初始化 Skill 相关组件
	skillsDir, err := defaultSkillsDir()
	if err != nil {
		return err
	}
	if a.cfg.App.SkillsDir != "" {
		skillsDir, err = expandSkillDir(a.cfg.App.SkillsDir)
		if err != nil {
			return err
		}
	}
	skillLoader := skill.NewLoader(skillsDir)
	skillsMiddleware := skill.NewSkillsMiddleware(skillLoader)
	skillRepo := skill.NewSkillRepository(a.postgresDB)
	skillSvc := skill.NewSkillService(skillRepo, skillLoader)
	installSvc := skill.NewInstallService(skillRepo, skillLoader)

	// 注册远程拉取源
	remote.RegisterSource(remote.NewGitHubProvider(""))

	skillCtrl := skillapi.NewController(skillSvc, skillLoader, installSvc)

	// 创建 agent 运行时（本地文件系统安全策略 + 全局后台任务管理器）
	// 安全策略构建后只读，可跨 agent 共享；后台任务管理器在应用关闭时统一回收。
	bgManager := backgroundtask.New(context.Background(), &backgroundtask.Config{})
	a.bgManager = bgManager
	// 安全策略构建后只读，跨 agent 运行时与 workspace 共享
	sharedPolicies := security.NewPolicies(&a.cfg.Agent.Security)
	// 审批中断 checkpoint 存储（Redis 可用时启用，否则可中断不可恢复）
	var checkPointStore adk.CheckPointStore
	if a.redis != nil {
		checkPointStore = agentpkg.NewRedisCheckPointStore(a.redis, 24*time.Hour)
	}

	agentRuntime := &agentpkg.AgentRuntime{
		Policies:            sharedPolicies,
		WorkspaceRoot:       a.cfg.Agent.WorkspaceRoot,
		SkillsDir:           skillsDir,
		ShellTimeoutSeconds: a.cfg.Agent.Security.ShellTimeoutSeconds,
		Background:          bgManager,
		Tracer:              tracer,
		CheckPointStore:     checkPointStore,
	}

	// 创建 AgentManager（modelSvc 实现 SubagentLLMResolver，用于子智能体 LLM 解析）
	agentMgr := agentpkg.NewAgentManager(mcpManager, vectorizer, toolRegistry, skillsMiddleware, agentSvc, agentRuntime, modelSvc)

	// 工作区 API：root = data/workspaces（已归一化为绝对路径），复用共享安全策略
	workspaceBackend := localfs.NewLocalBackend(a.cfg.Agent.WorkspaceRoot, sharedPolicies)
	workspaceSvc := service.NewWorkspaceService(workspaceBackend)
	workspaceCtrl := workspaceapi.NewController(workspaceSvc)

	// 创建短期记忆模块（需在 ConversationService 之前初始化，供 DeleteConversation 清理 Redis）
	shortTermStore := shortterm.NewRedisStore(a.redis, logger.GetLogger(), 24*time.Hour)
	shortTermBuffer := shortterm.NewMessageBufferManager(logger.GetLogger(), 20)
	shortTermState := shortterm.NewSessionStateManager(logger.GetLogger(), &modelResolverAdapter{modelSvc: modelSvc})
	// 摘要生成器：注入 ModelService 适配器，modelID 运行时动态解析为当前 Agent 使用的模型
	// modelID=0 时降级为规则式摘要（在 Worker 执行时可通过 Run 的 Agent 配置动态指定）
	shortTermSummary := shortterm.NewSummaryGenerator(logger.GetLogger(), nil, &modelResolverAdapter{modelSvc: modelSvc}, 0)
	shortTermMgr := shortterm.NewManager(shortTermStore, shortTermBuffer, shortTermState, shortTermSummary, logger.GetLogger())

	// 创建 Service
	conversationSvc := service.NewConversationService(conversationRepo, shortTermMgr, logger.GetLogger())
	messageSvc := service.NewMessageService(messageRepo, conversationRepo, a.redis)

	// 创建长期记忆模块（用户画像/偏好/决策/项目事实，跨会话持久化）
	// P0 阶段：全量注入 → P2 阶段：pgvector 语义检索 top-K
	ltmRepo := repository.NewLongTermMemoryRepository(a.postgresDB)
	longTermMgr := longterm.NewManager(logger.GetLogger(), ltmRepo,
		&modelResolverAdapter{modelSvc: modelSvc}, longterm.Config{
			MaxItems:      a.cfg.Memory.LongTerm.MaxItems,      // 召回上限（config.yaml: memory.long_term.max_items）
			MaxTokens:     a.cfg.Memory.LongTerm.MaxTokens,     // 注入 System Prompt 的最大 Token（memory.long_term.max_tokens）
			DefaultUserID: a.cfg.Memory.LongTerm.DefaultUserID, // JWT 未携带 UserID 时的降级用户（0 = 全局匿名共享池）
		})

	// 创建上下文管理器（集成 Short Memory + Long Term Memory）
	contextConfig := &contextmgr.ContextConfig{
		MaxTokens:     32768, // 上下文窗口（对话历史保留上限），中文每字约2-3 Token
		ReserveTokens: 4096,  // 预留给模型回复的 Token
		SystemPrompt:  "你是 Qavor AI 助手，请始终使用中文回答用户的问题。",
	}
	// 模型解析器适配器，用于根据 modelID 动态创建 LLM 客户端
	contextModelResolver := &contextModelResolverAdapter{modelSvc: modelSvc}
	contextMgr := contextmgr.NewContextManager(contextConfig, messageRepo, shortTermMgr, longTermMgr, nil, contextModelResolver, logger.GetLogger(), tracer)

	// 创建 SSE 模块
	heartbeatConfig := &sse.HeartbeatConfig{
		Interval:         30 * time.Second,
		BusinessInterval: 15 * time.Second,
		Timeout:          60 * time.Second,
	}
	heartbeatMgr := sse.NewHeartbeatManager(heartbeatConfig, logger.GetLogger())

	sseConfig := &sse.ManagerConfig{
		MaxConnectionsPerUser: 5,
		CleanInterval:         5 * time.Minute,
		ConnectionTimeout:     10 * time.Minute,
	}
	sseManager := sse.NewManager(heartbeatMgr, logger.GetLogger(), sseConfig)

	// 启动连接清理
	sseManager.StartCleaner(context.Background())

	// 创建 SSE API Controller (HTTP 处理)
	// 创建 Chat Service
	chatSvc := service.NewChatService(agentMgr, contextMgr, modelSvc, sseManager, messageRepo, conversationRepo, logger.GetLogger())

	// 创建 Chat Controller
	chatCtrl := chatctrl.NewController(chatSvc, tracer)

	// 创建 Agent Options Provider
	agentOpts := agentctrl.NewDefaultOptionsProvider(toolRegistry, mcpServerSvc, skillSvc, knowledgeBaseSvc, agentSvc)

	// —— Run 流式服务装配（POST 单连接流式 + Redis Stream 持久化）——
	var postStreamHandler *agentctrl.PostStreamHandler
	var runController *agentctrl.RunController
	// runRepo 在 Run 流式和 Trace 查询中都需要（Trace 用于补充 BusinessRunStatus）
	var runRepo repository.AgentRunRepository
	if a.postgresDB != nil {
		runRepo = repository.NewAgentRunRepository(a.postgresDB)
	}
	if a.redis != nil {
		blockDur := time.Duration(a.cfg.Run.BlockSeconds) * time.Second
		pub := eventbus.NewPublisher(a.redis, a.cfg.Run.StreamMaxLen)
		sub := eventbus.NewSubscriber(a.redis, blockDur)
		reqQueue := run.NewRequestQueue(a.redis,
			time.Duration(a.cfg.Run.LockTTLSeconds)*time.Second, blockDur)

		executor := run.NewAgentExecutor(agentMgr, modelSvc)
		todoStore := run.NewTodoStore(a.redis, 24*time.Hour)
		runWorker := run.NewWorker(reqQueue, pub, runRepo, messageRepo, conversationRepo, executor, contextMgr, longTermMgr, todoStore, modelSvc, logger.GetLogger(), a.cfg.Run.WorkerCount, tracer, a.cfg.Run.HeartbeatIntervalMs, a.cfg.Run.HeartbeatTimeoutSec)

		// 启动 Run Worker 池
		runWorkerCtx, cancelRunWorker := context.WithCancel(context.Background())
		a.runWorkerStop = cancelRunWorker
		go runWorker.Run(runWorkerCtx)
		logger.Info("Run Worker 已启动", zap.Int("worker_count", a.cfg.Run.WorkerCount))

		heartbeatPeriod := time.Duration(a.cfg.SSE.HeartbeatInterval) * time.Second
		postStreamHandler = agentctrl.NewPostStreamHandler(sub, runRepo, conversationRepo, reqQueue, heartbeatPeriod, logger.GetLogger(), tracer, traceSpanRepo)
		runController = agentctrl.NewRunController(runRepo, reqQueue, runWorker, logger.GetLogger(), contextMgr, conversationRepo, todoStore)
	} else {
		logger.Warn("Redis 不可用，Run 流式服务未启动")
	}

	// —— 链路追踪 Service/Controller/Janitor 装配（Tracer 已在上面创建）——
	var traceCtrl *tracectrl.Controller
	if a.cfg.Trace.Enabled && a.postgresDB != nil {
		traceSvc := service.NewTraceService(traceSpanRepo, runRepo)
		traceCtrl = tracectrl.NewController(traceSvc)
		jctx, cancelJanitor := context.WithCancel(context.Background())
		a.traceJanitorStop = cancelJanitor
		go trace.NewJanitor(traceSpanRepo,
			time.Duration(a.cfg.Trace.JanitorInterval)*time.Minute,
			time.Duration(a.cfg.Trace.TimeoutMinutes)*time.Minute,
		).Run(jctx)
		logger.Info("链路追踪已启用")
	}

	// 创建 Dashboard Service（只读统计查询）
	dashboardSvc := service.NewDashboardService(a.postgresDB)

	// —— RAG 评估服务（基准管理 + 评估运行，后台执行器随应用启动）——
	evaluationSvc := service.NewEvaluationService(
		repository.NewEvaluationRepository(a.postgresDB),
		knowledgeBaseRepo,
		knowledgeChunkRepo,
		ragSvc,
		modelSvc,
	)
	evaluationCtrl := evaluationctrl.NewController(evaluationSvc)
	a.evaluationSvc = evaluationSvc

	// 创建 Router
	a.router = api.NewRouter(authSvc, knowledgeBaseSvc, knowledgeFileSvc, processingJobSvc, modelSvc, conversationSvc, messageSvc, agentSvc, agentOpts, agentMgr, mcpManager, chatCtrl, ragCtrl, systemCtrl, toolRegistry, skillCtrl, mcpServerCtrl, postStreamHandler, runController, traceCtrl, dashboardSvc, workspaceCtrl, knowledgeQuerySvc, tracer, evaluationCtrl, mindmapCtrl, ocrCtrl)

	return nil
}

// initRouter 初始化路由
func (a *App) initRouter() {
	// 设置 Gin 模式
	gin.SetMode(a.cfg.App.Mode)
}

// initServer 初始化 HTTP 服务器
func (a *App) initServer() {
	engine := gin.New()

	// 注册路由
	a.router.Setup(engine)

	// 创建 HTTP 服务器
	a.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler:        engine,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
}

// buildMCPWhitelist 从所有 Agent 配置中提取需要的 MCP server name
func (a *App) buildMCPWhitelist(agentRepo repository.AgentRepository) []string {
	agents, _, err := agentRepo.List(0, 1000, "")
	if err != nil {
		logger.Warn("获取 Agent 列表失败，跳过 MCP 预热", zap.Error(err))
		return nil
	}

	seen := make(map[string]bool)
	var whitelist []string
	for _, ag := range agents {
		data, err := json.Marshal(ag.ConfigJSON)
		if err != nil {
			continue
		}
		var cfg struct {
			MCPServers []string `json:"mcp_servers"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		for _, name := range cfg.MCPServers {
			if !seen[name] {
				seen[name] = true
				whitelist = append(whitelist, name)
			}
		}
	}
	return whitelist
}

// Run 运行应用
func (a *App) Run() {
	// 启动 RAG 评估后台执行器（处理基准生成与评估运行任务）
	if a.evaluationSvc != nil {
		evalCtx, cancelEval := context.WithCancel(context.Background())
		a.evaluationStop = cancelEval
		a.evaluationSvc.Start(evalCtx)
		logger.Info("RAG 评估执行器已启动")
	}

	// 启动 HTTP 服务器
	go func() {
		logger.Info("HTTP 服务器启动",
			zap.String("addr", a.server.Addr),
			zap.String("mode", a.cfg.App.Mode),
		)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	// 优雅关闭
	a.gracefulShutdown()
}

// gracefulShutdown 优雅关闭
func (a *App) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error("服务器关闭失败", zap.Error(err))
	}
	if a.workerStop != nil {
		a.workerStop()
	}
	if a.workerDone != nil {
		select {
		case <-a.workerDone:
		case <-time.After(5 * time.Second):
			logger.Warn("等待文档处理 Worker 关闭超时")
		}
	}
	if a.runWorkerStop != nil {
		a.runWorkerStop()
		logger.Info("Run Worker 已关闭")
	}
	if a.traceJanitorStop != nil {
		a.traceJanitorStop()
		logger.Info("Trace Janitor 已关闭")
	}

	// 停止 RAG 评估执行器
	if a.evaluationStop != nil {
		a.evaluationStop()
		logger.Info("RAG 评估执行器已关闭")
	}

	// Flush 并关闭 Trace Writer（最多 3 秒）
	if a.traceWriter != nil {
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := a.traceWriter.Close(flushCtx); err != nil {
			logger.Warn("Trace Writer 关闭超时", zap.Error(err))
		}
		flushCancel()
		logger.Info("Trace Writer 已关闭")
	}

	// 关闭 MCP 服务器连接（SSE 断开、stdio 子进程终止等）
	if a.mcpManager != nil {
		a.mcpManager.CloseAll()
	}

	// 关闭 agent 后台任务管理器（终止运行中的后台任务并回收进程）
	if a.bgManager != nil {
		if err := a.bgManager.Close(ctx); err != nil {
			logger.Warn("关闭 agent 后台任务管理器失败", zap.Error(err))
		}
	}

	// 关闭数据库连接
	_ = database.ClosePostgres()
	_ = database.CloseRedis()

	// 关闭 MinIO
	_ = minio.Close()

	// 同步日志
	_ = logger.Sync()

	logger.Info("服务器已关闭")
	logger.Info("=========================================")
}

// modelResolverAdapter 将 service.ModelService 适配为 shortterm.ModelResolver
type modelResolverAdapter struct {
	modelSvc service.ModelService
}

func (a *modelResolverAdapter) CreateLLMClient(ctx context.Context, modelID uint) (shortterm.LLMClient, error) {
	client, err := a.modelSvc.CreateLLMClient(ctx, modelID)
	if err != nil {
		return nil, err
	}
	return &llmClientAdapter{client: client}, nil
}

// llmClientAdapter 将 llm.Client 适配为 context.LLMClient 和 shortterm.LLMClient
type llmClientAdapter struct {
	client   llm.Client
	modelSvc service.ModelService
	modelID  uint
}

func (a *llmClientAdapter) Generate(ctx context.Context, input []*schema.Message) (*schema.Message, error) {
	if a.client == nil {
		return nil, fmt.Errorf("LLM client not initialized")
	}
	return a.client.Generate(ctx, input)
}

// ocrEngineForParser 读取默认 OCR 引擎与通用 OCR API 配置，供文档解析执行分派。
// 返回 (引擎标识, API 地址, API Key, 模型名)；引擎标识取值 "rapidocr"（本地，默认）或 "api"。
func ocrEngineForParser(ctx context.Context, settings repository.SystemSettingRepository, configSvc service.SystemConfigService) (engine, apiBaseURL, apiKey, apiModel string) {
	defaultEngine, _, err := settings.Get(ctx, service.SettingKeyDefaultOCREngine)
	if err != nil || defaultEngine != "api_ocr" {
		return "rapidocr", "", "", ""
	}
	cfg, err := configSvc.GetOCRAPIConfig(ctx)
	if err != nil {
		logger.Warn("读取 OCR API 配置失败，回退本地 RapidOCR", zap.Error(err))
		return "rapidocr", "", "", ""
	}
	if cfg.BaseURL == "" {
		logger.Warn("默认 OCR 引擎为 api_ocr 但未配置服务地址，回退本地 RapidOCR")
		return "rapidocr", "", "", ""
	}
	return "api", cfg.BaseURL, cfg.APIKey, cfg.Model
}

// contextModelResolverAdapter 将 service.ModelService 适配为 context.ModelResolver
type contextModelResolverAdapter struct {
	modelSvc service.ModelService
}

func (a *contextModelResolverAdapter) CreateLLMClient(ctx context.Context, modelID uint) (llm.Client, error) {
	return a.modelSvc.CreateLLMClient(ctx, modelID)
}

func (a *contextModelResolverAdapter) GetContextWindow(modelID uint) int {
	if _, _, contextWindow, ok := a.modelSvc.GetModelInfo(modelID); ok {
		return contextWindow
	}
	return 0
}
