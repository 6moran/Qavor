package entity

// ConversationStats 对话统计实体
type ConversationStats struct {
	BaseEntity
	ConversationID uint   `gorm:"uniqueIndex;not null;comment:对话ID" json:"conversation_id"`
	MessageCount   int    `gorm:"default:0;comment:消息总数" json:"message_count"`
	TotalTokens    int    `gorm:"default:0;comment:Token总消耗" json:"total_tokens"`
	ModelUsed      string `gorm:"type:varchar(100);comment:使用的模型" json:"model_used,omitempty"`
	UserFeedback   JSON   `gorm:"type:json;comment:用户反馈" json:"user_feedback,omitempty"`

	// 关联关系
	Conversation *Conversation `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
}

// TableName 指定表名
func (ConversationStats) TableName() string {
	return "conversation_stats"
}
