package response

import (
	"Qavor/internal/agent"
	"time"
)

// AgentResponse 智能体响应
type AgentResponse struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	IsDefault   bool              `json:"is_default"`
	Config      agent.AgentConfig `json:"config"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AgentListResponse 智能体列表响应
type AgentListResponse struct {
	Total int64           `json:"total"`
	Items []AgentResponse `json:"items"`
}
