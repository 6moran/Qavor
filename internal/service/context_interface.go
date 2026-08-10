package service

import (
	"context"

	ctxpkg "Qavor/internal/context"

	"github.com/cloudwego/eino/schema"
)

// ContextManager 上下文管理器接口（简化版，用于 Chat Controller）
type ContextManager interface {
	// UpdateShortMemory 更新短期记忆（AI回复完成后调用）
	// modelID 指定摘要/状态抽取使用的模型，0 时降级为规则式
	UpdateShortMemory(ctx context.Context, conversationID uint, message *schema.Message, modelID uint) error

	// GetShortMemoryContext 获取短期记忆上下文
	GetShortMemoryContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error)

	// GetAgentState 获取 Agent 状态面板数据
	GetAgentState(ctx context.Context, conversationID uint) (*ctxpkg.AgentState, error)
}
