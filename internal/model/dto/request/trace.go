package request

// TraceListRequest Trace 列表请求
type TraceListRequest struct {
	Keyword        string `form:"keyword" binding:"omitempty,max=100"`
	AgentSlug      string `form:"agent_slug" binding:"omitempty"`
	ConversationID uint   `form:"conversation_id" binding:"omitempty"`
	Status         string `form:"status" binding:"omitempty,oneof=running ok error cancelled interrupted timeout"`
	Model          string `form:"model" binding:"omitempty"`
	Tool           string `form:"tool" binding:"omitempty"`
	ErrorOnly      bool   `form:"error_only" binding:"omitempty"`
	MismatchOnly   bool   `form:"mismatch_only" binding:"omitempty"`
	From           string `form:"from" binding:"omitempty"`
	To             string `form:"to" binding:"omitempty"`
	Page           int    `form:"page" binding:"omitempty,min=1"`
	PageSize       int    `form:"page_size" binding:"omitempty,min=1,max=100"`
}
