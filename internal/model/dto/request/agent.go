package request

import "Qavor/internal/model/entity"

// CreateAgentRequest 创建智能体请求
type CreateAgentRequest struct {
	Name        string           `json:"name" binding:"required,max=100"`
	BackendID   string           `json:"backend_id" binding:"required,max=64"`
	Slug        string           `json:"slug" binding:"omitempty,max=80"`
	Description string           `json:"description" binding:"omitempty"`
	Icon        string           `json:"icon" binding:"omitempty,max=255"`
	Pics        entity.JSONArray `json:"pics" binding:"omitempty"`
	ConfigJSON  entity.JSON      `json:"config_json" binding:"omitempty"`
	IsSubagent  *bool            `json:"is_subagent" binding:"omitempty"`
}

// UpdateAgentRequest 更新智能体请求
type UpdateAgentRequest struct {
	Name        string           `json:"name" binding:"omitempty,max=100"`
	Description string           `json:"description" binding:"omitempty"`
	Icon        string           `json:"icon" binding:"omitempty,max=255"`
	Pics        entity.JSONArray `json:"pics" binding:"omitempty"`
	ConfigJSON  entity.JSON      `json:"config_json" binding:"omitempty"`
}

// AgentListRequest 智能体列表请求
type AgentListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword" binding:"omitempty"`
}
