package request

// TraceListRequest Trace 列表请求
type TraceListRequest struct {
	Keyword        string `form:"keyword" binding:"omitempty,max=100"`
	AgentSlug      string `form:"agent_slug" binding:"omitempty"`
	ConversationID uint   `form:"conversation_id" binding:"omitempty"`
	Status         string `form:"status" binding:"omitempty,oneof=running success failed cancelled timeout"`
	Source         string `form:"source" binding:"omitempty,oneof=sync stream run"`
	From           string `form:"from" binding:"omitempty"`
	To             string `form:"to" binding:"omitempty"`
	Page           int    `form:"page" binding:"omitempty,min=1"`
	PageSize       int    `form:"page_size" binding:"omitempty,min=1,max=100"`
}
