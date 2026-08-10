package context

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// ContextManager 上下文管理器接口
type ContextManager interface {
	// LoadHistory 加载对话历史（含短期记忆摘要），返回裁剪后的消息列表
	LoadHistory(ctx context.Context, conversationID uint) ([]*schema.Message, error)

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

	// UpdateShortMemory 更新短期记忆（AI回复完成后调用）
	// modelID 指定摘要/状态抽取使用的模型，0 时降级为规则式
	UpdateShortMemory(ctx context.Context, conversationID uint, message *schema.Message, modelID uint) error

	// GetShortMemoryContext 获取短期记忆上下文
	GetShortMemoryContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error)

	// GetAgentState 获取 Agent 状态面板数据（token 用量、待办、文件、子 Agent 运行）
	GetAgentState(ctx context.Context, conversationID uint) (*AgentState, error)
}
