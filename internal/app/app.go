package app

import (
	agentpkg "Qavor/internal/agent"
	"Qavor/internal/api"
	agentctrl "Qavor/internal/api/v1/agent"
	chatctrl "Qavor/internal/api/v1/chat"
	mcpserverctrl "Qavor/internal/api/v1/mcp_server"
	ragctrl "Qavor/internal/api/v1/rag"
	ssectrl "Qavor/internal/api/v1/sse"
	contextmgr "Qavor/internal/context"
	"Qavor/internal/ingestion"
	"Qavor/internal/llm"
	"Qavor/internal/mcp"
	"Qavor/internal/model/entity"
	documentqueue "Qavor/internal/queue"
	"Qavor/internal/rag"
	"Qavor/internal/repository"
	"Qavor/internal/service"
	"Qavor/internal/skill"
	skillapi "Qavor/internal/skill/api"
	"Qavor/internal/skill/remote"
	"Qavor/internal/store"
	"Qavor/internal/tool"
	"Qavor/internal/tool/builtin"
	"Qavor/internal/worker"
	"Qavor/pkg/config"
	"Qavor/pkg/database"
	"Qavor/pkg/logger"
	"Qavor/pkg/minio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

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

// App 应用结构体
type App struct {
	cfg        *config.Config
	postgresDB *gorm.DB
	redis      *redis.Client
	router     *api.Router
	server     *http.Server
	workerStop context.CancelFunc
	workerDone chan struct{}
	mcpManager *mcp.MCPManager
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
			&entity.AgentEnv{},
			&entity.Conversation{},
			&entity.ConversationStats{},
			&entity.Message{},
			&entity.MessageFeedback{},
			&entity.ToolCall{},
			&entity.APIKey{},
			&entity.OperationLog{},
			&entity.AgentRun{},
			&entity.SubagentThread{},
			&entity.TaskRecord{},
			&entity.Model{},
			&entity.Skill{},
			&entity.KnowledgeBase{},
			&entity.KnowledgeFile{},
			&entity.KnowledgeChunk{},
			&entity.DocumentProcessingJob{},
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
	knowledgeBaseSvc := service.NewKnowledgeBaseService(knowledgeBaseRepo, modelRepo)
	storage := service.NewMinIOObjectStorage()
	knowledgeFileSvc := service.NewKnowledgeFileService(knowledgeBaseRepo, knowledgeFileRepo, processingJobRepo, storage, queue)
	processingJobSvc := service.NewProcessingJobService(processingJobRepo, knowledgeFileRepo, queue)
	agentSvc := service.NewAgentService(agentRepo)

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
		answerer rag.AnswerChain = rag.NewDynamicAnswerEngine(
			knowledgeBaseRepo,
			modelSvc,
			knowledgeChunkRepo,
			a.cfg.RAG,
		)
		ragCtrl *ragctrl.Controller
	)
	ragSvc := service.NewRAGService(a.cfg.RAG, knowledgeBaseRepo, answerer)
	ragCtrl = ragctrl.NewController(ragSvc, a.cfg.RAG.RequestTimeoutSeconds)

	if queue != nil {
		workerCtx, cancelWorker := context.WithCancel(context.Background())
		a.workerStop = cancelWorker
		a.workerDone = make(chan struct{})
		parser := ingestion.NewParser(ingestion.NewPythonParser("python", "pkg/documentparser/python/parse_document.py"))
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

	// 创建 ToolVectorizer（预留，embedder 为 nil 时不启用向量检索）
	vectorizer := mcp.NewToolVectorizer(mcpManager, nil)

	// 创建 ToolRegistry
	toolRegistry := tool.NewDefaultRegistry()
	toolProvider := builtin.NewBuiltinToolProvider()
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
	skillResolver := skill.NewResolver(skillLoader, toolRegistry, mcpManager)
	activation := skill.NewActivationState()
	skillsMiddleware := skill.NewSkillsMiddleware(skillLoader, skillResolver, activation)
	skillRepo := skill.NewSkillRepository(a.postgresDB)
	skillSvc := skill.NewSkillService(skillRepo, skillLoader)
	installSvc := skill.NewInstallService(skillRepo, skillLoader)

	// 注册远程拉取源
	remote.RegisterSource(remote.NewGitHubProvider(""))

	skillCtrl := skillapi.NewController(skillSvc, skillLoader, installSvc)

	// 创建 AgentManager
	agentMgr := agentpkg.NewAgentManager(mcpManager, vectorizer, toolRegistry, skillsMiddleware, skillResolver, agentSvc)

	// 创建 Service
	conversationSvc := service.NewConversationService(conversationRepo)
	messageSvc := service.NewMessageService(messageRepo, conversationRepo, a.redis)

	// 创建 Context Manager
	contextConfig := &contextmgr.ContextConfig{
		MaxTokens:     4096,
		ReserveTokens: 1024,
		SystemPrompt:  "You are a helpful assistant.",
	}
	contextMgr := contextmgr.NewContextManager(contextConfig, messageRepo, logger.GetLogger())

	// 创建 SSE Service（从配置文件读取）
	llmFactory := llm.NewClient
	sseSvc := service.NewSSEService(contextMgr, llmFactory, &a.cfg.SSE, logger.GetLogger())

	// 创建 SSE API Controller (HTTP 处理)
	sseAPICtrl := ssectrl.NewController(sseSvc, &a.cfg.SSE, logger.GetLogger())

	// 创建 Chat Controller
	chatCtrl := chatctrl.NewController(agentMgr, modelSvc, messageRepo, conversationSvc, messageSvc)

	// 创建 Agent Options Provider
	agentOpts := agentctrl.NewDefaultOptionsProvider(toolRegistry, mcpServerSvc, skillSvc, knowledgeBaseSvc, agentSvc)

	// 创建 Router
	a.router = api.NewRouter(authSvc, knowledgeBaseSvc, knowledgeFileSvc, processingJobSvc, modelSvc, conversationSvc, messageSvc, agentSvc, agentOpts, chatCtrl, ragCtrl, toolRegistry, skillCtrl, sseAPICtrl, mcpServerCtrl)

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

	// 关闭 MCP 服务器连接（SSE 断开、stdio 子进程终止等）
	if a.mcpManager != nil {
		a.mcpManager.CloseAll()
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
