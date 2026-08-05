package response

import (
	"Qavor/internal/agent"
	"Qavor/internal/model/entity"
	"time"
)

// AgentResponse 智能体响应
type AgentResponse struct {
	Slug              string                      `json:"slug"`
	Name              string                      `json:"name"`
	Description       string                      `json:"description,omitempty"`
	IsDefault         bool                        `json:"is_default"`
	BackendID         string                      `json:"backend_id,omitempty"`
	IsSubagent        bool                        `json:"is_subagent"`
	Config            agent.AgentConfig           `json:"config"`
	ConfigJSON        entity.JSON                 `json:"config_json,omitempty"`
	ConfigurableItems map[string]ConfigurableItem `json:"configurable_items,omitempty"`
	CreatedAt         time.Time                   `json:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at"`
}

// ConfigurableItem 前端可配置项 schema
type ConfigurableItem struct {
	Name        string                   `json:"name,omitempty"`
	Kind        string                   `json:"kind"`
	Type        string                   `json:"type,omitempty"`
	Description string                   `json:"description,omitempty"`
	Default     interface{}              `json:"default,omitempty"`
	Options     []map[string]interface{} `json:"options,omitempty"`
}

// AgentListResponse 智能体列表响应
type AgentListResponse struct {
	Total int64           `json:"total"`
	Items []AgentResponse `json:"items"`
}
