package request

// CreateAgentRequest 创建智能体请求
type CreateAgentRequest struct {
	Slug          string            `json:"slug" binding:"required,max=80"`
	Name          string            `json:"name" binding:"required,max=100"`
	Description   string            `json:"description"`
	Icon          string            `json:"icon"`
	Instruction   string            `json:"instruction"`
	ProviderID    string            `json:"provider_id"`
	ModelName     string            `json:"model_name"`
	Tools         []string          `json:"tools"`
	DisabledTools []string          `json:"disabled_tools"`
	MaxTokens     int               `json:"max_tokens"`
	Temperature   float64           `json:"temperature"`
	Metadata      map[string]string `json:"metadata"`
	IsDefault     bool              `json:"is_default"`
}

// UpdateAgentRequest 更新智能体请求
type UpdateAgentRequest struct {
	Name          *string           `json:"name"`
	Description   *string           `json:"description"`
	Icon          *string           `json:"icon"`
	Instruction   *string           `json:"instruction"`
	ProviderID    *string           `json:"provider_id"`
	ModelName     *string           `json:"model_name"`
	Tools         []string          `json:"tools"`
	DisabledTools []string          `json:"disabled_tools"`
	MaxTokens     *int              `json:"max_tokens"`
	Temperature   *float64          `json:"temperature"`
	Metadata      map[string]string `json:"metadata"`
}

// AgentListRequest 智能体列表请求
type AgentListRequest struct {
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"page_size,default=20"`
	Keyword  string `form:"keyword"`
}
