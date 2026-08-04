package shortterm

import (
	"time"
)

// SessionMemory 会话短期记忆（内存中的表示）
type SessionMemory struct {
	ConversationID uint           `json:"conversation_id"`
	Buffer         *MessageBuffer `json:"buffer"`  // 消息缓冲区
	Summary        string         `json:"summary"` // 上下文摘要
	State          *SessionState  `json:"state"`   // 会话状态
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// MessageBuffer 消息缓冲区
type MessageBuffer struct {
	Messages    []BufferMessage `json:"messages"`     // 缓存的消息
	MaxSize     int             `json:"max_size"`     // 最大消息数
	TotalTokens int             `json:"total_tokens"` // 估算总 Token 数
}

// BufferMessage 缓冲消息
type BufferMessage struct {
	MessageID      string            `json:"message_id"`         // 消息唯一标识
	Role           string            `json:"role"`               // 消息角色
	Content        string            `json:"content"`            // 消息内容
	Timestamp      time.Time         `json:"timestamp"`          // 时间戳
	Tokens         int               `json:"tokens"`             // 估算 Token 数
	ConversationID uint              `json:"conversation_id"`    // 会话ID
	Metadata       map[string]string `json:"metadata,omitempty"` // 元数据
}

// SessionState 会话状态
type SessionState struct {
	Topic       string            `json:"topic"`        // 当前讨论主题
	UserIntent  string            `json:"user_intent"`  // 用户意图
	KeyEntities []string          `json:"key_entities"` // 关键实体
	Metadata    map[string]string `json:"metadata"`     // 其他元数据
}

// SummaryConfig 摘要配置
type SummaryConfig struct {
	MessageThreshold int  // 消息数量阈值
	TokenThreshold   int  // Token 阈值
	EnableAsync      bool // 是否启用异步生成
}

// DefaultSummaryConfig 默认摘要配置
func DefaultSummaryConfig() *SummaryConfig {
	return &SummaryConfig{
		MessageThreshold: 20,
		TokenThreshold:   8000,
		EnableAsync:      true,
	}
}
