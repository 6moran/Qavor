package agent

import (
	"context"

	"Qavor/internal/mcp"

	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// AgentManager 管理 Agent 实例创建
type AgentManager struct {
	mcpManager *mcp.MCPManager
}

// NewAgentManager 创建 AgentManager
func NewAgentManager(mcpManager *mcp.MCPManager) *AgentManager {
	return &AgentManager{
		mcpManager: mcpManager,
	}
}

// Create 根据配置创建 Agent（不缓存，每次用最新配置）
func (m *AgentManager) Create(ctx context.Context, cfg *AgentConfig, llm model.ToolCallingChatModel) (*QavorAgent, error) {
	a, err := NewQavorAgent(ctx, cfg, llm, m.mcpManager)
	if err != nil {
		logger.Error("创建 Agent 失败", zap.String("name", cfg.Name), zap.Error(err))
		return nil, err
	}
	return a, nil
}
