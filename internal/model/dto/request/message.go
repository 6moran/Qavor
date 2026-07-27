package request

import "Qavor/internal/model/entity"

// CreateMessageRequest 创建消息请求
type CreateMessageRequest struct {
	ConversationID uint        `json:"conversation_id" binding:"required"`
	Role           string      `json:"role" binding:"required,oneof=user assistant system tool"`
	Content        string      `json:"content" binding:"required"`
	MessageType    string      `json:"message_type" binding:"omitempty,oneof=text tool_call tool_result"`
	ImageContent   string      `json:"image_content" binding:"omitempty"`
	ExtraMetadata  entity.JSON `json:"extra_metadata" binding:"omitempty"`
}

// MessageListRequest 消息列表请求
type MessageListRequest struct {
	Page           int    `form:"page" binding:"omitempty,min=1"`
	PageSize       int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	ConversationID uint   `form:"conversation_id" binding:"required"`
	Role           string `form:"role" binding:"omitempty,oneof=user assistant system tool"`
}
