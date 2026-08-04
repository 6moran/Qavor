package shortterm

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// Manager 短期记忆管理器接口
type Manager interface {
	// GetMemory 获取会话的短期记忆
	GetMemory(ctx context.Context, conversationID uint) (*SessionMemory, error)

	// UpdateMemory 更新短期记忆（AI回复完成后异步调用）
	UpdateMemory(ctx context.Context, conversationID uint, message *schema.Message) error

	// GetContext 获取用于 LLM 的上下文（包含摘要+最近消息）
	GetContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error)

	// ClearMemory 清除会话的短期记忆
	ClearMemory(ctx context.Context, conversationID uint) error

	// RefreshTTL 刷新会话的 Redis TTL
	RefreshTTL(ctx context.Context, conversationID uint) error
}
