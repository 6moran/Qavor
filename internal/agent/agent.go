package agent

import (
	"context"

	"Qavor/internal/mcp"
	"Qavor/internal/tool"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
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
func NewQavorAgent(ctx context.Context, cfg *AgentConfig, llm model.ToolCallingChatModel, mcpManager *mcp.MCPManager, toolRegistry *tool.Registry, query string, vectorizer *mcp.ToolVectorizer) (*QavorAgent, error) {
	// 获取 MCP 工具
	mcpTools := mcpManager.GetTools()

	// 获取内置工具（根据白名单过滤）
	var builtinTools []einotool.BaseTool
	if len(cfg.Tools) > 0 {
		builtinTools = toolRegistry.ToEinoToolsByNames(cfg.Tools)
	} else {
		builtinTools = toolRegistry.ToEinoTools()
	}

	// 合并工具：内置工具 + MCP 工具
	allTools := append(builtinTools, mcpTools...)

	// 过滤工具（静态过滤 + 向量检索）
	// 注意：内置工具不参与向量检索
	tools := filterTools(ctx, allTools, cfg, query, vectorizer)

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

// filterTools 过滤工具：静态过滤（白名单/黑名单）+ 向量检索
func filterTools(ctx context.Context, allTools []einotool.BaseTool, cfg *AgentConfig, query string, vectorizer *mcp.ToolVectorizer) []einotool.BaseTool {
	// 1. 静态过滤（白名单/黑名单）
	disabledSet := make(map[string]bool)
	for _, name := range cfg.DisabledTools {
		disabledSet[name] = true
	}
	enabledSet := make(map[string]bool)
	for _, name := range cfg.Tools {
		enabledSet[name] = true
	}

	var staticFiltered []einotool.BaseTool
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
		staticFiltered = append(staticFiltered, t)
	}

	// 2. 向量检索（如果启用且工具数超过阈值）
	if vectorizer != nil && cfg.ToolRetrievalEnabled && len(staticFiltered) > cfg.ToolRetrievalThreshold && query != "" {
		names := vectorizer.SelectTools(ctx, query, cfg.ToolRetrievalTopK)
		if names != nil {
			nameSet := make(map[string]bool, len(names))
			for _, n := range names {
				nameSet[n] = true
			}
			var result []einotool.BaseTool
			for _, t := range staticFiltered {
				info, err := t.Info(ctx)
				if err != nil {
					continue
				}
				if nameSet[info.Name] {
					result = append(result, t)
				}
			}
			return result
		}
	}

	return staticFiltered
}
