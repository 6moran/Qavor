package request

// CreateConversationRequest 创建对话请求
type CreateConversationRequest struct {
	AgentID string `json:"agent_id" binding:"required,max=64"`
	Title   string `json:"title" binding:"omitempty,max=255"`
}

// UpdateConversationRequest 更新对话请求
type UpdateConversationRequest struct {
	Title    string `json:"title" binding:"omitempty,max=255"`
	IsPinned *bool  `json:"is_pinned" binding:"omitempty"`
}

// ConversationListRequest 对话列表请求
type ConversationListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Status   string `form:"status" binding:"omitempty,oneof=active archived deleted"`
	AgentID  string `form:"agent_id" binding:"omitempty"`
	Query    string `form:"q" binding:"omitempty,max=100"`
}
