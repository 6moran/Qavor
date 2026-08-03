package agent

import (
	"context"
	"fmt"

	"Qavor/internal/mcp"
	"Qavor/internal/tool"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Agent Qavor 智能体
type Agent struct {
	agent      *adk.ChatModelAgent
	mcpManager *mcp.MCPManager
	config     *AgentConfig
}

// AgentResponse 智能体响应
type AgentResponse struct {
	Content string
}

// NewAgent 创建智能体（构造一次，复用）
func NewAgent(cfg *AgentConfig, llm model.ToolCallingChatModel,
	mcpManager *mcp.MCPManager, toolRegistry *tool.Registry,
	vectorizer *mcp.ToolVectorizer, skillTools map[string][]einotool.BaseTool) (*Agent, error) {

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
	if len(cfg.Tools) > 0 {
		builtinTools = toolRegistry.ToEinoToolsByNames(cfg.Tools)
	}

	// 创建工具过滤中间件（始终注册，低于阈值时直接透传）
	vectorCfg := RetrievalConfig{
		Enabled:   cfg.ToolRetrievalEnabled,
		Threshold: cfg.ToolRetrievalThreshold,
		TopK:      cfg.ToolRetrievalTopK,
	}
	middleware := NewToolFilterMiddleware(builtinTools, mcpTools, skillTools, vectorizer, vectorCfg)

	// 创建 adk Agent（工具列表为空，由中间件注入）
	agentConfig := &adk.ChatModelAgentConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Instruction: cfg.Instruction,
		Model:       llm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: nil,
			},
		},
		Handlers: []adk.ChatModelAgentMiddleware{middleware},
	}

	a, err := adk.NewChatModelAgent(context.Background(), agentConfig)
	if err != nil {
		return nil, err
	}

	return &Agent{
		agent:      a,
		mcpManager: mcpManager,
		config:     cfg,
	}, nil
}

// Execute 执行智能体
func (a *Agent) Execute(ctx context.Context, query string) (*AgentResponse, error) {
	ctx = WithQuery(ctx, query)

	input := &adk.AgentInput{
		Messages: []*schema.Message{
			{
				Role:    schema.User,
				Content: query,
			},
		},
	}

	iter := a.agent.Run(ctx, input)
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

// GetMCPManager 获取 MCP 管理器
func (a *Agent) GetMCPManager() *mcp.MCPManager {
	return a.mcpManager
}

// GetConfig 获取配置
func (a *Agent) GetConfig() *AgentConfig {
	return a.config
}
