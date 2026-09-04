package shortterm

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// Manager 短期记忆管理器接口
type Manager interface {
	// GetMemory 获取会话的短期上下文
	GetMemory(ctx context.Context, conversationID uint) (*SessionContext, error)

	// UpdateMemory 更新短期上下文（消息到达时异步调用）
	// modelID 指定摘要/状态抽取使用的模型，0 时降级为规则式
	UpdateMemory(ctx context.Context, conversationID uint, message *schema.Message, modelID uint) error

	// GetContext 获取用于 LLM 的上下文（包含摘要+最近消息）
	GetContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error)

	// ClearMemory 清除会话的短期上下文
	ClearMemory(ctx context.Context, conversationID uint) error

	// GetMetrics 获取监控指标快照
	GetMetrics() *MemoryMetricsSnapshot
}
