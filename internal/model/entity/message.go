package entity

// Message 消息实体
type Message struct {
	BaseEntity
	ConversationID uint   `gorm:"not null;index;comment:所属对话ID" json:"conversation_id"`
	Role           string `gorm:"type:varchar(20);not null;comment:角色：user/assistant/system/tool" json:"role"`
	Content        string `gorm:"type:text;not null;comment:消息内容" json:"content"`
	MessageType    string `gorm:"type:varchar(30);default:text;comment:消息类型：text/tool_call/tool_result" json:"message_type"`
	TokenCount     *int   `gorm:"comment:Token消耗数量" json:"token_count,omitempty"`
	ExtraMetadata  JSON   `gorm:"type:json;comment:附加元数据（完整消息快照）" json:"extra_metadata,omitempty"`
	ImageContent   string `gorm:"type:text;comment:Base64编码的图片内容" json:"image_content,omitempty"`
	RunID          string `gorm:"type:varchar(64);index;comment:关联的Agent Run ID" json:"run_id,omitempty"`
	RequestID      string `gorm:"type:varchar(64);index;comment:请求ID（幂等性保证）" json:"request_id,omitempty"`
	DeliveryStatus string `gorm:"type:varchar(32);not null;default:complete;comment:投递状态" json:"delivery_status"`

	// 关联关系
	Conversation *Conversation     `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
	ToolCalls    []ToolCall        `gorm:"foreignKey:MessageID;constraint:OnDelete:CASCADE" json:"tool_calls,omitempty"`
	Feedbacks    []MessageFeedback `gorm:"foreignKey:MessageID;constraint:OnDelete:CASCADE" json:"feedbacks,omitempty"`
}

// TableName 指定表名
func (Message) TableName() string {
	return "messages"
}
