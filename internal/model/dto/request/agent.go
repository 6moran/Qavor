package request

import "Qavor/internal/model/entity"

// CreateAgentRequest 创建智能体请求（只提交基本信息，配置通过后续编辑设置）
type CreateAgentRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
	ModelID     string `json:"model_id"`
	BackendID   string `json:"backend_id" binding:"required"`
}

// UpdateAgentRequest 更新智能体请求（基本信息 + 配置分离）
type UpdateAgentRequest struct {
	Name        *string     `json:"name"`
	Description *string     `json:"description"`
	Instruction *string     `json:"instruction"`
	ModelID     *string     `json:"model_id"`
	ConfigJSON  entity.JSON `json:"config_json"`
}

// AgentListRequest 智能体列表请求
type AgentListRequest struct {
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"page_size,default=20"`
	Keyword  string `form:"keyword"`
}
