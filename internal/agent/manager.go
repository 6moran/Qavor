package agent

import (
	"context"

	"Qavor/internal/mcp"
	"Qavor/internal/tool"

	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// AgentManager 管理 Agent 实例创建
type AgentManager struct {
	mcpManager   *mcp.MCPManager
	vectorizer   *mcp.ToolVectorizer
	toolRegistry *tool.Registry
}

// NewAgentManager 创建 AgentManager
func NewAgentManager(mcpManager *mcp.MCPManager, vectorizer *mcp.ToolVectorizer, toolRegistry *tool.Registry) *AgentManager {
	return &AgentManager{
		mcpManager:   mcpManager,
		vectorizer:   vectorizer,
		toolRegistry: toolRegistry,
	}
}

// Create 根据配置创建 Agent
func (m *AgentManager) Create(ctx context.Context, cfg *AgentConfig, llm model.ToolCallingChatModel, query string) (*QavorAgent, error) {
	a, err := NewQavorAgent(ctx, cfg, llm, m.mcpManager, m.toolRegistry, query, m.vectorizer)
	if err != nil {
		logger.Error("创建 Agent 失败", zap.String("name", cfg.Name), zap.Error(err))
		return nil, err
	}
	return a, nil
}
