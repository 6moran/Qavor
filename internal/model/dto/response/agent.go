package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// AgentResponse 智能体响应
type AgentResponse struct {
	ID          uint             `json:"id"`
	Slug        string           `json:"slug"`
	BackendID   string           `json:"backend_id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Icon        string           `json:"icon,omitempty"`
	Pics        entity.JSONArray `json:"pics"`
	ConfigJSON  entity.JSON      `json:"config_json"`
	ShareConfig entity.JSON      `json:"share_config"`
	IsDefault   bool             `json:"is_default"`
	IsSubagent  bool             `json:"is_subagent"`
	CreatedBy   string           `json:"created_by,omitempty"`
	UpdatedBy   string           `json:"updated_by,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// AgentListResponse 智能体列表响应
type AgentListResponse struct {
	Total int64           `json:"total"`
	Items []AgentResponse `json:"items"`
}
