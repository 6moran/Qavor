package agent

import (
	"context"
	"fmt"

	"Qavor/internal/mcp"
	"Qavor/internal/skill"
	"Qavor/internal/tool"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Agent Qavor 智能体
type Agent struct {
	agent      adk.ResumableAgent
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

// NewAgent 创建智能体（构造一次，复用）
func NewAgent(cfg *AgentConfig, llm model.ToolCallingChatModel,
	mcpManager *mcp.MCPManager, toolRegistry *tool.Registry,
	vectorizer *mcp.ToolVectorizer,
	skillsMiddleware *skill.SkillsMiddleware) (*Agent, error) {

	if cfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}
	if llm == nil {
		return nil, fmt.Errorf("llm is required")
	}

	// 获取 MCP 工具（只获取配置的服务器）
	var mcpTools []einotool.BaseTool
	if len(cfg.MCPServers) > 0 {
		mcpTools = mcpManager.GetToolsByServers(cfg.MCPServers)
	}

	// 获取内置工具
	var builtinTools []einotool.BaseTool
	if names := configuredBuiltinToolNames(cfg); len(names) > 0 {
		builtinTools = toolRegistry.ToEinoToolsByNames(names)
	}

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

	// 组装中间件列表：工具过滤 + Skill 激活检测
	handlers := []adk.TypedChatModelAgentMiddleware[*schema.Message]{middleware}
	if skillsMiddleware != nil {
		handlers = append(handlers, skillsMiddleware)
	}

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
		return &Agent{
			agent:      a,
			mcpManager: mcpManager,
			config:     cfg,
		}, nil
	}

	// 主智能体：使用 Deep Agent，具备任务分解、todo 管理、子智能体编排能力
	// 文件系统工具与 Shell 执行硬编码启用（主智能体始终具备完整工具能力）

	// 通用子智能体默认启用，用户可显式关闭
	enableGeneralSubAgent := true
	if cfg.EnableGeneralSubAgent != nil {
		enableGeneralSubAgent = *cfg.EnableGeneralSubAgent
	}

	// 沙箱目录按 slug 隔离：data/sandboxes/<slug>
	sandboxRoot := "data/sandboxes"
	if cfg.Slug != "" {
		sandboxRoot = sandboxRoot + "/" + cfg.Slug
	}
	//sandbox, err := NewDiskSandbox(sandboxRoot)
	//if err != nil {
	//	return nil, err
	//}

	deepConfig := &deep.Config{
		Name:                   cfg.Name,
		Description:            cfg.Description,
		ChatModel:              llm,
		Instruction:            cfg.Instruction,
		ToolsConfig:            adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: nil}},
		MaxIteration:           maxIteration,
		WithoutGeneralSubAgent: !enableGeneralSubAgent,
		//Backend:                sandbox,               // 磁盘沙箱（read_file/write_file/edit_file/glob/grep）
		//StreamingShell:         &DefaultStreamingShell{}, // 流式 Shell（execute）
		Handlers: handlers,
	}

	a, err := deep.New(context.Background(), deepConfig)
	if err != nil {
		return nil, err
	}

	return &Agent{
		agent:      a,
		mcpManager: mcpManager,
		config:     cfg,
	}, nil
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

	input := &adk.AgentInput{
		Messages: []*schema.Message{
			{
				Role:    schema.User,
				Content: query,
			},
		},
	}

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

	iter := a.agent.Run(ctx, input, runOpts...)
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

	input := &adk.AgentInput{
		Messages: []*schema.Message{
			{
				Role:    schema.User,
				Content: query,
			},
		},
	}

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

	iter := a.agent.Run(ctx, input, runOpts...)
	return &AgentEventIterator{iter: iter}
}

// GetMCPManager 获取 MCP 管理器
func (a *Agent) GetMCPManager() *mcp.MCPManager {
	return a.mcpManager
}

// GetConfig 获取配置
func (a *Agent) GetConfig() *AgentConfig {
	return a.config
}
