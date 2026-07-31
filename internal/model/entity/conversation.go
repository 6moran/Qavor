package entity

// Conversation 对话实体
type Conversation struct {
	BaseEntity
	UserID        uint   `gorm:"not null;index;comment:用户ID" json:"user_id"`
	ThreadID      string `gorm:"type:varchar(64);uniqueIndex;not null;comment:对话线程ID（UUID）" json:"thread_id"`
	AgentID       string `gorm:"type:varchar(64);not null;index;comment:Agent slug" json:"agent_id"`
	Title         string `gorm:"type:varchar(255);comment:对话标题" json:"title,omitempty"`
	Status        string `gorm:"type:varchar(20);default:active;comment:状态：active/archived/deleted" json:"status"`
	IsPinned      bool   `gorm:"not null;default:false;index;comment:是否置顶" json:"is_pinned"`
	ExtraMetadata JSON   `gorm:"type:json;comment:附加元数据" json:"extra_metadata,omitempty"`

	// 关联关系
	Messages []Message          `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
	Stats    *ConversationStats `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE" json:"stats,omitempty"`
}

// TableName 指定表名
func (Conversation) TableName() string {
	return "conversations"
}
