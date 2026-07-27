package entity

// MCPServer MCP服务器配置实体
type MCPServer struct {
	BaseEntity
	Slug           string    `gorm:"type:varchar(100);uniqueIndex;not null;comment:稳定标识" json:"slug"`
	Name           string    `gorm:"type:varchar(100);not null;comment:展示名称" json:"name"`
	Description    string    `gorm:"type:varchar(500);comment:描述" json:"description,omitempty"`
	Transport      string    `gorm:"type:varchar(20);not null;comment:传输类型：sse/streamable_http/stdio" json:"transport"`
	URL            string    `gorm:"type:varchar(500);comment:服务器URL（sse/streamable_http）" json:"url,omitempty"`
	Command        string    `gorm:"type:varchar(500);comment:命令（stdio）" json:"command,omitempty"`
	Args           JSONArray `gorm:"type:json;comment:命令参数数组（stdio）" json:"args,omitempty"`
	Env            JSON      `gorm:"type:json;comment:环境变量（stdio）" json:"env,omitempty"`
	Headers        JSON      `gorm:"type:json;comment:HTTP请求头" json:"headers,omitempty"`
	Timeout        *int      `gorm:"comment:HTTP超时时间（秒）" json:"timeout,omitempty"`
	SSEReadTimeout *int      `gorm:"comment:SSE读取超时（秒）" json:"sse_read_timeout,omitempty"`
	Tags           JSONArray `gorm:"type:json;comment:标签数组" json:"tags,omitempty"`
	Icon           string    `gorm:"type:varchar(50);comment:图标（emoji）" json:"icon,omitempty"`
	Enabled        int       `gorm:"not null;default:1;comment:是否启用：1=是，0=否" json:"enabled"`
	DisabledTools  JSONArray `gorm:"type:json;comment:禁用的工具名称列表" json:"disabled_tools,omitempty"`
	CreatedBy      string    `gorm:"type:varchar(100);not null;comment:创建人用户名" json:"created_by"`
	UpdatedBy      string    `gorm:"type:varchar(100);not null;comment:修改人用户名" json:"updated_by"`
}

// TableName 指定表名
func (MCPServer) TableName() string {
	return "mcp_servers"
}

// ToMCPConfig 转换为MCP配置格式
func (m *MCPServer) ToMCPConfig() map[string]interface{} {
	config := map[string]interface{}{
		"transport": m.Transport,
	}

	switch m.Transport {
	case "sse", "streamable_http":
		if m.URL != "" {
			config["url"] = m.URL
		}
		if m.Headers != nil {
			config["headers"] = m.Headers
		}
		if m.Timeout != nil {
			config["timeout"] = *m.Timeout
		}
	case "stdio":
		if m.Command != "" {
			config["command"] = m.Command
		}
		if m.Args != nil {
			config["args"] = m.Args
		}
		if m.Env != nil {
			config["env"] = m.Env
		}
	}

	return config
}
