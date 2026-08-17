package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// MCPServerResponse MCP服务器响应
type MCPServerResponse struct {
	ID             uint             `json:"id"`
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
	Enabled        int              `json:"enabled"`
	DisabledTools  entity.JSONArray `json:"disabled_tools,omitempty"`
	Status         string           `json:"status"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// MCPServerListResponse MCP服务器列表响应
type MCPServerListResponse struct {
	Total int64               `json:"total"`
	Items []MCPServerResponse `json:"items"`
}

// MCPToolResponse MCP工具响应
type MCPToolResponse struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// MCPTestResponse MCP连接测试响应
type MCPTestResponse struct {
	ServerName    string `json:"server_name,omitempty"`
	ServerVersion string `json:"server_version,omitempty"`
}
