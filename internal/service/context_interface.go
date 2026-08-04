package service

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// ContextManager 上下文管理器接口（简化版，用于 Chat Controller）
type ContextManager interface {
	// UpdateShortMemory 更新短期记忆（AI回复完成后调用）
	UpdateShortMemory(ctx context.Context, conversationID uint, message *schema.Message) error

	// GetShortMemoryContext 获取短期记忆上下文
	GetShortMemoryContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error)
}
