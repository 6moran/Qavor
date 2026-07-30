package agent

import (
	"context"

	"Qavor/internal/mcp"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// QavorAgent Qavor 智能体
type QavorAgent struct {
	agent      *adk.ChatModelAgent
	mcpManager *mcp.MCPManager
	config     *AgentConfig
}

// NewQavorAgent 创建智能体
func NewQavorAgent(ctx context.Context, cfg *AgentConfig, llm model.ToolCallingChatModel, mcpManager *mcp.MCPManager) (*QavorAgent, error) {
	// 获取并过滤 MCP 工具
	allTools := mcpManager.GetTools()
	tools := filterTools(ctx, allTools, cfg)

	// 创建 adk Agent
	agentConfig := &adk.ChatModelAgentConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Instruction: cfg.Instruction,
		Model:       llm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
	}

	a, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, err
	}

	return &QavorAgent{
		agent:      a,
		mcpManager: mcpManager,
		config:     cfg,
	}, nil
}

// Chat 对话
func (a *QavorAgent) Chat(ctx context.Context, message string) (string, error) {
	input := &adk.AgentInput{
		Messages: []*schema.Message{
			{
				Role:    schema.User,
				Content: message,
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
			return "", event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.Message != nil {
			result = event.Output.MessageOutput.Message.Content
		}
	}

	return result, nil
}

// ChatWithHistory 带历史记录的对话
func (a *QavorAgent) ChatWithHistory(ctx context.Context, messages []*schema.Message) (string, error) {
	input := &adk.AgentInput{
		Messages: messages,
	}

	iter := a.agent.Run(ctx, input)
	var result string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.Message != nil {
			result = event.Output.MessageOutput.Message.Content
		}
	}

	return result, nil
}

// GetMCPManager 获取 MCP 管理器
func (a *QavorAgent) GetMCPManager() *mcp.MCPManager {
	return a.mcpManager
}

// GetConfig 获取配置
func (a *QavorAgent) GetConfig() *AgentConfig {
	return a.config
}

// filterTools 根据配置过滤工具
func filterTools(ctx context.Context, allTools []tool.BaseTool, cfg *AgentConfig) []tool.BaseTool {
	if len(cfg.Tools) == 0 && len(cfg.DisabledTools) == 0 {
		return allTools
	}

	disabledSet := make(map[string]bool)
	for _, name := range cfg.DisabledTools {
		disabledSet[name] = true
	}

	enabledSet := make(map[string]bool)
	for _, name := range cfg.Tools {
		enabledSet[name] = true
	}

	var result []tool.BaseTool
	for _, t := range allTools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		name := info.Name
		if disabledSet[name] {
			continue
		}
		if len(enabledSet) > 0 && !enabledSet[name] {
			continue
		}
		result = append(result, t)
	}
	return result
}
