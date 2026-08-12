package response

import (
	"time"

	"Qavor/internal/model/entity"
)

// ConversationResponse 对话响应
type ConversationResponse struct {
	ID        uint         `json:"id"`
	ThreadID  string       `json:"thread_id"`
	AgentID   string       `json:"agent_id"`
	Title     string       `json:"title,omitempty"`
	Status    string       `json:"status"`
	IsPinned  bool         `json:"is_pinned"`
	Metadata  entity.JSON  `json:"metadata,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// ConversationListResponse 对话列表响应
type ConversationListResponse struct {
	Total int64                  `json:"total"`
	Items []ConversationResponse `json:"items"`
}

// ConversationDetailResponse 对话详情响应
type ConversationDetailResponse struct {
	ConversationResponse
	Messages []MessageResponse          `json:"messages,omitempty"`
	Stats    *ConversationStatsResponse `json:"stats,omitempty"`
}

// ConversationStatsResponse 对话统计响应
type ConversationStatsResponse struct {
	ID             uint        `json:"id"`
	ConversationID uint        `json:"conversation_id"`
	MessageCount   int         `json:"message_count"`
	TotalTokens    int         `json:"total_tokens"`
	ModelUsed      string      `json:"model_used,omitempty"`
	UserFeedback   interface{} `json:"user_feedback,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}
