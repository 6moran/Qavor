package entity

// SubagentThread 子智能体线程关系实体
type SubagentThread struct {
	BaseEntity
	UID                  string `gorm:"type:varchar(64);not null;index;comment:用户UID" json:"uid"`
	ParentConversationID uint   `gorm:"not null;index;comment:父对话ID" json:"parent_conversation_id"`
	ChildConversationID  uint   `gorm:"uniqueIndex;not null;index;comment:子对话ID" json:"child_conversation_id"`
	ChildThreadID        string `gorm:"type:varchar(64);uniqueIndex;not null;index;comment:子线程ID" json:"child_thread_id"`
	SubagentSlug         string `gorm:"type:varchar(64);not null;index;comment:子智能体slug" json:"subagent_slug"`
	CreatedByRunID       string `gorm:"type:varchar(64);not null;index;comment:创建该子线程的Run ID" json:"created_by_run_id"`

	// 关联关系
	ParentConversation *Conversation `gorm:"foreignKey:ParentConversationID" json:"parent_conversation,omitempty"`
	ChildConversation  *Conversation `gorm:"foreignKey:ChildConversationID" json:"child_conversation,omitempty"`
}

// TableName 指定表名
func (SubagentThread) TableName() string {
	return "subagent_threads"
}
