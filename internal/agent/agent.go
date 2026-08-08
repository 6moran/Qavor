package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"Qavor/internal/agent/localfs"
	"Qavor/internal/agent/localfs/security"
	"Qavor/internal/mcp"
	"Qavor/internal/skill"
	"Qavor/internal/tool"
	"Qavor/pkg/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/backgroundtask"
	fs2 "github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// Agent Qavor 智能体
type Agent struct {
	agent      adk.ResumableAgent
	runner     *adk.Runner // 审批中断/恢复执行器（带 CheckPointStore）
	mcpManager *mcp.MCPManager
	config     *AgentConfig
}

// AgentResponse 智能体响应
type AgentResponse struct {
	Content string
}

// configuredBuiltinToolNames 返回实际启用的内置工具名：
// 配置了知识库时自动追加 query_kb，且不修改 cfg.Tools。
func configuredBuiltinToolNames(cfg *AgentConfig) []string {
	names := append([]string(nil), cfg.Tools...)
	if len(cfg.Knowledges) > 0 {
		found := false
		for _, name := range names {
			if name == tool.QueryKBToolName {
				found = true
				break
			}
		}
		if !found {
			names = append(names, tool.QueryKBToolName)
		}
	}
	return names
}

// resolveAgentTools 解析指定配置的内置工具与 MCP 工具。
// 主智能体与子智能体共用：按各自配置独立解析，互不污染。
func resolveAgentTools(cfg *AgentConfig, mcpManager *mcp.MCPManager, toolRegistry *tool.Registry) ([]einotool.BaseTool, []einotool.BaseTool) {
	var mcpTools []einotool.BaseTool
	if len(cfg.MCPServers) > 0 {
		mcpTools = mcpManager.GetToolsByServers(cfg.MCPServers)
	}
	var builtinTools []einotool.BaseTool
	if names := configuredBuiltinToolNames(cfg); len(names) > 0 {
		builtinTools = toolRegistry.ToEinoToolsByNames(names)
	}
	return builtinTools, mcpTools
}

// NewAgent 创建智能体（构造一次，复用）。
// subagents 为主智能体挂载的子智能体 specs（由 AgentManager 组装）；仅主智能体使用。
func NewAgent(cfg *AgentConfig, llm model.ToolCallingChatModel,
	mcpManager *mcp.MCPManager, toolRegistry *tool.Registry,
	vectorizer *mcp.ToolVectorizer,
	skillsMiddleware *skill.SkillsMiddleware,
	runtime *AgentRuntime,
	subagents []*subagentSpec) (*Agent, error) {

	if cfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}
	if llm == nil {
		return nil, fmt.Errorf("llm is required")
	}

	// 获取工具（主智能体）
	builtinTools, mcpTools := resolveAgentTools(cfg, mcpManager, toolRegistry)

	// 创建工具过滤中间件（始终注册，低于阈值时直接透传）
	vectorCfg := RetrievalConfig{
		Enabled:   cfg.ToolRetrievalEnabled,
		Threshold: cfg.ToolRetrievalThreshold,
		TopK:      cfg.ToolRetrievalTopK,
	}
	middleware := NewToolFilterMiddleware(builtinTools, mcpTools, vectorizer, vectorCfg)

	// 从配置读取 MaxIteration，默认值为 20
	maxIteration := 20
	if cfg.MaxIteration > 0 {
		maxIteration = cfg.MaxIteration
	}

	// 组装中间件列表：工具过滤 + Skill 激活检测 + 工具审批
	handlers := []adk.TypedChatModelAgentMiddleware[*schema.Message]{middleware}
	if skillsMiddleware != nil {
		handlers = append(handlers, skillsMiddleware)
	}
	handlers = append(handlers, NewApprovalMiddleware())

	// 子智能体：使用轻量的 ChatModelAgent，专注执行任务
	// （不携带 deep 的 write_todos/task/general 子编排工具，避免无意义的递归嵌套）
	if cfg.IsSubagent {
		a, err := adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
			Name:        cfg.Name,
			Description: cfg.Description,
			Instruction: cfg.Instruction,
			Model:       llm,
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: nil, // 工具列表为空，由中间件注入
				},
			},
			MaxIterations: maxIteration,
			Handlers:      handlers,
		})
		if err != nil {
			return nil, err
		}
		return newAgent(a, mcpManager, cfg, runtime), nil
	}

	// 主智能体：使用 Deep Agent，具备任务分解、todo 管理、子智能体编排能力
	// 文件系统工具与 Shell 执行硬编码启用（主智能体始终具备完整工具能力）

	// 通用子智能体默认启用，用户可显式关闭
	enableGeneralSubAgent := true
	if cfg.EnableGeneralSubAgent != nil {
		enableGeneralSubAgent = *cfg.EnableGeneralSubAgent
	}

	// 本地文件系统与 Shell（安全管控内聚于 localfs）。
	// filesystem 中间件手动构造（newFilesystemMiddleware），以注入自定义 execute 描述；
	// deepConfig 的 Backend/StreamingShell 保持 nil，避免 builtin 二次注册。
	var backgroundCfg *deep.BackgroundConfig
	var deepSubagents []adk.TypedAgent[*schema.Message]
	if runtime != nil {
		// 工作目录按 slug 隔离：data/workspaces/<slug>
		workDir := runtime.WorkspaceRoot
		if cfg.Slug != "" {
			workDir = filepath.Join(runtime.WorkspaceRoot, cfg.Slug)
		}
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			return nil, fmt.Errorf("创建 agent 工作目录失败: %w", err)
		}
		// 将关联 Skill 暴露到工作区 skills/<slug>，使 read_file 能读到 SKILL.md
		exposeSkills(workDir, skillsMiddleware, cfg.Skills)
		if runtime.Background != nil {
			backgroundCfg = &deep.BackgroundConfig{
				Manager:   runtime.Background,
				OutputDir: filepath.Join(workDir, "background_output"),
			}
		}
		fsMW, err := newFilesystemMiddleware(workDir, runtime.SkillsDir, runtime.Policies,
			runtime.Background, time.Duration(runtime.ShellTimeoutSeconds)*time.Second)
		if err != nil {
			return nil, fmt.Errorf("创建文件系统中间件失败: %w", err)
		}
		handlers = append(handlers, fsMW)

		// 为每个子智能体构造实例（共享主 fsMW：backend/shell/工作区）。
		// 单个子智能体构造失败仅记录 warning，不阻塞主智能体可用性。
		for _, spec := range subagents {
			sub, err := buildSubagentInstance(spec, fsMW, mcpManager, toolRegistry, vectorizer, skillsMiddleware)
			if err != nil {
				warnLog("构造子智能体失败，跳过", zap.String("slug", spec.cfg.Slug), zap.Error(err))
				continue
			}
			deepSubagents = append(deepSubagents, sub)
		}
	}

	deepConfig := &deep.Config{
		Name:                   cfg.Name,
		Description:            cfg.Description,
		ChatModel:              llm,
		Instruction:            cfg.Instruction,
		ToolsConfig:            adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: nil}},
		MaxIteration:           maxIteration,
		WithoutGeneralSubAgent: !enableGeneralSubAgent,
		Background:             backgroundCfg,
		SubAgents:              deepSubagents,
		Handlers:               handlers,
	}

	a, err := deep.New(context.Background(), deepConfig)
	if err != nil {
		return nil, err
	}

	return newAgent(a, mcpManager, cfg, runtime), nil
}

// newAgent 构造 Agent 实例并创建 runner。
// runner 带 CheckPointStore（来自 runtime），支持审批中断/恢复。
// CheckPointStore 为 nil 时 runner 仍可创建（中断不可恢复，仅中断不恢复）。
func newAgent(a adk.ResumableAgent, mcpManager *mcp.MCPManager, cfg *AgentConfig, runtime *AgentRuntime) *Agent {
	ag := &Agent{
		agent:      a,
		mcpManager: mcpManager,
		config:     cfg,
	}
	var store adk.CheckPointStore
	if runtime != nil {
		store = runtime.CheckPointStore
	}
	ag.runner = adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:           a,
		EnableStreaming: true,
		CheckPointStore: store,
	})
	return ag
}

// newFilesystemMiddleware 构造带自定义 execute 描述的文件系统中间件。
// backend/shell 的安全管控（根目录隔离、技能符号链接白名单、敏感文件守卫、
// 高危命令黑名单、输出脱敏）内聚于 localfs。
func newFilesystemMiddleware(workDir, skillsDir string, policies *security.Policies,
	bg *backgroundtask.Manager, shellTimeout time.Duration) (adk.TypedChatModelAgentMiddleware[*schema.Message], error) {

	// SetSkillsRoot 是 *LocalBackend 的方法（非 filesystem.Backend 接口），
	// 先收进 *LocalBackend 局部变量设置技能符号链接白名单，再赋给中间件配置。
	backend := localfs.NewLocalBackend(workDir, policies)
	backend.SetSkillsRoot(skillsDir)
	shell := localfs.NewLocalStreamingShell(workDir, policies, shellTimeout)
	// 给模型描述真实工作区路径与根隔离边界：
	// - execute 用 buildExecuteToolDesc 填入 workDir（原本是含 <slug> 字面量的死模板）
	// - 文件工具覆写 eino 默认描述（"能读机器上所有文件"会误导模型越界尝试）
	execDesc := buildExecuteToolDesc(workDir)
	fsDesc := buildFsToolDesc(workDir)
	mwCfg := &fs2.MiddlewareConfig{
		Backend:        backend,
		StreamingShell: shell,
		ExecuteToolConfig: &fs2.ExecuteToolConfig{
			ToolConfig: fs2.ToolConfig{Desc: &execDesc},
		},
		LsToolConfig:        &fs2.ToolConfig{Desc: &fsDesc},
		ReadFileToolConfig:  &fs2.ToolConfig{Desc: &fsDesc},
		WriteFileToolConfig: &fs2.ToolConfig{Desc: &fsDesc},
		EditFileToolConfig:  &fs2.ToolConfig{Desc: &fsDesc},
		GlobToolConfig:      &fs2.ToolConfig{Desc: &fsDesc},
		GrepToolConfig:      &fs2.ToolConfig{Desc: &fsDesc},
	}
	if bg != nil {
		mwCfg.Background = &fs2.BackgroundConfig{
			Manager:     bg,
			OutputStore: backend, // LocalBackend 实现 AppendOpener，后台任务输出落盘
			OutputDir:   filepath.Join(workDir, "background_output"),
		}
	}
	return fs2.NewTyped[*schema.Message](context.Background(), mwCfg)
}

// executionContext 绑定查询与知识库范围，供一次 Agent Run 使用。
// 知识库范围只来自配置，LLM 无法通过工具参数指定。
func (a *Agent) executionContext(ctx context.Context, query string) context.Context {
	ctx = WithQuery(ctx, query)
	if len(a.config.Knowledges) > 0 {
		ctx = tool.WithKnowledgeBaseIDs(ctx, a.config.Knowledges)
	}
	return ctx
}

// Execute 执行智能体
func (a *Agent) Execute(ctx context.Context, query string) (*AgentResponse, error) {
	ctx = a.executionContext(ctx, query)

	// 构建模型调用选项（temperature、max_tokens）
	var modelOpts []model.Option
	if a.config.Temperature != nil {
		modelOpts = append(modelOpts, model.WithTemperature(float32(*a.config.Temperature)))
	}
	if a.config.MaxTokens != nil {
		modelOpts = append(modelOpts, model.WithMaxTokens(*a.config.MaxTokens))
	}

	// 创建运行选项
	var runOpts []adk.AgentRunOption
	if len(modelOpts) > 0 {
		runOpts = append(runOpts, adk.WithChatModelOptions(modelOpts))
	}

	messages := []*schema.Message{{Role: schema.User, Content: query}}
	iter := a.runner.Run(ctx, messages, runOpts...)
	var result string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return nil, event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.Message != nil {
			result = event.Output.MessageOutput.Message.Content
		}
	}

	return &AgentResponse{Content: result}, nil
}

// AgentEventIterator Agent 事件迭代器
type AgentEventIterator struct {
	iter interface {
		Next() (*adk.AgentEvent, bool)
	}
}

// Next 获取下一个事件
func (it *AgentEventIterator) Next() (*adk.AgentEvent, bool) {
	return it.iter.Next()
}

// ExecuteIter 执行智能体并返回事件迭代器（用于流式输出）
func (a *Agent) ExecuteIter(ctx context.Context, query string) *AgentEventIterator {
	ctx = a.executionContext(ctx, query)

	// 构建模型调用选项（temperature、max_tokens）
	var modelOpts []model.Option
	if a.config.Temperature != nil {
		modelOpts = append(modelOpts, model.WithTemperature(float32(*a.config.Temperature)))
	}
	if a.config.MaxTokens != nil {
		modelOpts = append(modelOpts, model.WithMaxTokens(*a.config.MaxTokens))
	}

	// 创建运行选项
	var runOpts []adk.AgentRunOption
	if len(modelOpts) > 0 {
		runOpts = append(runOpts, adk.WithChatModelOptions(modelOpts))
	}

	messages := []*schema.Message{{Role: schema.User, Content: query}}
	iter := a.runner.Run(ctx, messages, runOpts...)
	return &AgentEventIterator{iter: iter}
}

// Resume 从审批中断点恢复执行。
// checkpointID 为中断时保存的 checkpoint key；targets 为 中断地址→用户决定 的映射
// （approve 放行 / reject 拒绝）。
func (a *Agent) Resume(ctx context.Context, checkpointID string, targets map[string]any) (*AgentEventIterator, error) {
	iter, err := a.runner.ResumeWithParams(ctx, checkpointID, &adk.ResumeParams{Targets: targets})
	if err != nil {
		return nil, err
	}
	return &AgentEventIterator{iter: iter}, nil
}

// GetMCPManager 获取 MCP 管理器
func (a *Agent) GetMCPManager() *mcp.MCPManager {
	return a.mcpManager
}

// GetConfig 获取配置
func (a *Agent) GetConfig() *AgentConfig {
	return a.config
}

// exposeSkills 将关联的 Skill 目录暴露到 agent 工作区 skills/<slug> 下，
// 使 read_file(path="skills/<slug>/SKILL.md") 可直接读取（渐进式披露 L2）。
// 优先符号链接（源目录变更即时同步），失败时回退为复制目录。
func exposeSkills(workDir string, skillsMiddleware *skill.SkillsMiddleware, slugs []string) {
	if skillsMiddleware == nil {
		return
	}
	loader := skillsMiddleware.GetLoader()
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		src := loader.GetSkillDir(slug)
		if _, err := os.Stat(src); err != nil {
			warnLog("Skill 目录不存在，跳过暴露", zap.String("slug", slug), zap.Error(err))
			continue
		}
		dst := filepath.Join(workDir, "skills", slug)
		if _, err := os.Lstat(dst); err == nil {
			continue // 已暴露
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			warnLog("创建 Skill 暴露目录失败", zap.String("slug", slug), zap.Error(err))
			continue
		}
		absSrc, absErr := filepath.Abs(src)
		if absErr == nil {
			src = absSrc
		}
		if err := os.Symlink(src, dst); err != nil {
			if err := copyDir(src, dst); err != nil {
				warnLog("暴露 Skill 到工作区失败", zap.String("slug", slug), zap.Error(err))
			}
		}
	}
}

// copyDir 递归复制目录（符号链接不可用时的回退方案）
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			return err
		}
		if err := os.WriteFile(d, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// warnLog 在 logger 已初始化时记录警告，避免未初始化导致 panic
func warnLog(msg string, fields ...zap.Field) {
	if logger.Initialized() {
		logger.Warn(msg, fields...)
	}
}
