package entity

import "time"

// MCPServerConfig MCP Server 配置（文件存储）
type MCPServerConfig struct {
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Transport      string            `json:"transport"`
	URL            string            `json:"url,omitempty"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Timeout        *int              `json:"timeout,omitempty"`
	SSEReadTimeout *int              `json:"sseReadTimeout,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	Icon           string            `json:"icon,omitempty"`
	Enabled        bool              `json:"enabled"`
	DisabledTools  []string          `json:"disabledTools,omitempty"`
	CreatedBy      string            `json:"createdBy"`
	UpdatedBy      string            `json:"updatedBy"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}
