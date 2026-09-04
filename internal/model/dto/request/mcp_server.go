package request

import "Qavor/internal/model/entity"

// CreateMCPServerRequest 创建MCP服务器请求
type CreateMCPServerRequest struct {
	Name           string           `json:"name" binding:"required,max=100"`
	Description    string           `json:"description" binding:"omitempty,max=500"`
	Transport      string           `json:"transport" binding:"required,oneof=sse streamable_http stdio"`
	URL            string           `json:"url" binding:"omitempty,max=500"`
	Command        string           `json:"command" binding:"omitempty,max=500"`
	Args           entity.JSONArray `json:"args" binding:"omitempty"`
	Env            entity.JSON      `json:"env" binding:"omitempty"`
	Headers        entity.JSON      `json:"headers" binding:"omitempty"`
	Timeout        *int             `json:"timeout" binding:"omitempty,min=1"`
	SSEReadTimeout *int             `json:"sse_read_timeout" binding:"omitempty,min=1"`
}

// UpdateMCPServerRequest 更新MCP服务器请求
type UpdateMCPServerRequest struct {
	Name           string           `json:"name" binding:"omitempty,max=100"`
	Description    string           `json:"description" binding:"omitempty,max=500"`
	URL            string           `json:"url" binding:"omitempty,max=500"`
	Command        string           `json:"command" binding:"omitempty,max=500"`
	Args           entity.JSONArray `json:"args" binding:"omitempty"`
	Env            entity.JSON      `json:"env" binding:"omitempty"`
	Headers        entity.JSON      `json:"headers" binding:"omitempty"`
	Timeout        *int             `json:"timeout" binding:"omitempty,min=1"`
	SSEReadTimeout *int             `json:"sse_read_timeout" binding:"omitempty,min=1"`
	Enabled        *int             `json:"enabled" binding:"omitempty,oneof=0 1"`
}

// MCPServerListRequest MCP服务器列表请求
type MCPServerListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword" binding:"omitempty"`
}
