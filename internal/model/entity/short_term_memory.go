package entity

import (
	"time"
)

// ShortTermMemory 短期记忆实体
type ShortTermMemory struct {
	BaseEntity
	ConversationID uint   `gorm:"not null;index;uniqueIndex;comment:会话ID" json:"conversation_id"`
	Summary        string `gorm:"type:text;comment:上下文摘要" json:"summary"`
	State          JSON   `gorm:"type:json;comment:会话状态" json:"state"`
	TotalTokens    int    `gorm:"default:0;comment:估算总Token数" json:"total_tokens"`
}

// TableName 指定表名
func (ShortTermMemory) TableName() string {
	return "short_term_memories"
}

// ShortTermMemoryMessage 短期记忆消息关联表
type ShortTermMemoryMessage struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	MemoryID  uint      `gorm:"not null;index;comment:短期记忆ID" json:"memory_id"`
	MessageID string    `gorm:"type:varchar(64);not null;index;comment:消息唯一标识" json:"message_id"`
	Role      string    `gorm:"type:varchar(20);not null;comment:消息角色" json:"role"`
	Content   string    `gorm:"type:text;not null;comment:消息内容" json:"content"`
	Tokens    int       `gorm:"default:0;comment:估算Token数" json:"tokens"`
	Metadata  JSON      `gorm:"type:json;comment:元数据" json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (ShortTermMemoryMessage) TableName() string {
	return "short_term_memory_messages"
}

// SessionState 会话状态结构
type SessionState struct {
	Topic       string            `json:"topic"`        // 当前讨论主题
	UserIntent  string            `json:"user_intent"`  // 用户意图
	KeyEntities []string          `json:"key_entities"` // 关键实体
	Metadata    map[string]string `json:"metadata"`     // 其他元数据
}
