package context

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// ContextManager 上下文管理器接口
type ContextManager interface {
	// FetchContext 提取历史与记忆（步骤1）
	FetchContext(ctx context.Context, query *ContextHistoryQuery) (*ContextWindow, error)

	// CompressContext 裁剪与压缩（步骤2）
	CompressContext(ctx context.Context, window *ContextWindow) (*ContextWindow, error)

	// BuildPrompt 组装 Prompt（步骤3）
	BuildPrompt(ctx context.Context, window *ContextWindow, userMessage *schema.Message) []*schema.Message

	// PersistUserMessage 收到请求时同步先落库
	PersistUserMessage(ctx context.Context, conversationID uint, userMsg *schema.Message) (uint, error)

	// PersistAssistantMessage LLM 响应后保存回复
	PersistAssistantMessage(ctx context.Context, conversationID uint, assistantMsg *schema.Message) error

	// CountTokens 计算消息列表的 Token 数量
	CountTokens(messages []*schema.Message) int
}
