package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// MessageResponse 消息响应
type MessageResponse struct {
	ID               uint               `json:"id"`
	ConversationID   uint               `json:"conversation_id"`
	Role             string             `json:"role"`
	Content          string             `json:"content"`
	ReasoningContent string             `json:"reasoning_content,omitempty"`
	MessageType      string             `json:"message_type"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	TokenCount       *int               `json:"token_count,omitempty"`
	ImageContent     string             `json:"image_content,omitempty"`
	ExtraMetadata    entity.JSON        `json:"extra_metadata,omitempty"`
	RunID            string             `json:"run_id,omitempty"`
	RequestID        string             `json:"request_id,omitempty"`
	DeliveryStatus   string             `json:"delivery_status"`
	IsFinished       bool               `json:"is_finished"`
	ToolCalls        []ToolCallResponse `json:"tool_calls,omitempty"`
}

// MessageListResponse 消息列表响应
type MessageListResponse struct {
	Total int64             `json:"total"`
	Items []MessageResponse `json:"items"`
}

// ToolCallResponse 工具调用响应
type ToolCallResponse struct {
	ID                  uint        `json:"id"`
	MessageID           uint        `json:"message_id"`
	LanggraphToolCallID string      `json:"langgraph_tool_call_id,omitempty"`
	ToolName            string      `json:"tool_name"`
	ToolInput           entity.JSON `json:"tool_input,omitempty"`
	ToolOutput          string      `json:"tool_output,omitempty"`
	Status              string      `json:"status"`
	ErrorMessage        string      `json:"error_message,omitempty"`
	CreatedAt           time.Time   `json:"created_at"`
}
