package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// MCPServerResponse MCP服务器响应
type MCPServerResponse struct {
	ID             uint             `json:"id"`
	Slug           string           `json:"slug"`
	Name           string           `json:"name"`
	Description    string           `json:"description,omitempty"`
	Transport      string           `json:"transport"`
	URL            string           `json:"url,omitempty"`
	Command        string           `json:"command,omitempty"`
	Args           entity.JSONArray `json:"args,omitempty"`
	Env            entity.JSON      `json:"env,omitempty"`
	Headers        entity.JSON      `json:"headers,omitempty"`
	Timeout        *int             `json:"timeout,omitempty"`
	SSEReadTimeout *int             `json:"sse_read_timeout,omitempty"`
	Tags           entity.JSONArray `json:"tags,omitempty"`
	Icon           string           `json:"icon,omitempty"`
	Enabled        int              `json:"enabled"`
	DisabledTools  entity.JSONArray `json:"disabled_tools,omitempty"`
	CreatedBy      string           `json:"created_by"`
	UpdatedBy      string           `json:"updated_by"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// MCPServerListResponse MCP服务器列表响应
type MCPServerListResponse struct {
	Total int64               `json:"total"`
	Items []MCPServerResponse `json:"items"`
}
