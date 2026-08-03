package context

import (
	"github.com/cloudwego/eino/schema"
)

// ContextConfig 上下文配置
type ContextConfig struct {
	MaxTokens      int    // 模型最大 Token 限制（如 4096、8192、128000）
	ReserveTokens  int    // 预留给回复的 Token 数量
	SystemPrompt   string // 系统提示词
	TokenizerModel string // Token 计数使用的模型标识
}

// ContextWindow 上下文窗口
type ContextWindow struct {
	Messages        []*schema.Message // 裁剪后的消息列表
	TotalTokens     int               // 消息总 Token 数
	TrimmedCount    int               // 被裁剪的消息数量
	HasSystem       bool              // 是否包含系统消息
	MemoryContext   string            // 长期记忆上下文（可选）
	RAGContext      string            // RAG 检索结果（可选）
	ToolDefinitions []interface{}     // 工具定义（可选）
}

// ContextHistoryQuery 历史查询参数
type ContextHistoryQuery struct {
	ConversationID uint     // 会话 ID
	Limit          int      // 最大消息数量
	BeforeID       uint     // 分页：此 ID 之前的消息（游标分页）
	Roles          []string // 过滤角色（可选）
}
