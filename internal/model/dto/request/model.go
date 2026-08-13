package request

// ModelParams 模型默认推理参数
type ModelParams struct {
	MaxTokens        int      `json:"max_tokens"`
	Temperature      float64  `json:"temperature"`
	TopP             float64  `json:"top_p"`
	PresencePenalty  float64  `json:"presence_penalty"`
	FrequencyPenalty float64  `json:"frequency_penalty"`
	Stop             []string `json:"stop,omitempty"`
}

// CreateModelRequest 创建模型请求
type CreateModelRequest struct {
	Name      string            `json:"name" binding:"required,max=100"`
	Remark    string            `json:"remark" binding:"omitempty,max=255"`
	Protocol  string            `json:"protocol" binding:"required,max=32"`
	BaseURL   string            `json:"base_url" binding:"required,max=500"`
	APIKey    string            `json:"api_key" binding:"omitempty,max=500"`
	Headers   map[string]string `json:"headers" binding:"omitempty"`
	Timeout   int               `json:"timeout" binding:"omitempty,min=1000,max=300000"`
	Enabled   *bool             `json:"enabled" binding:"omitempty"`
	ModelType string            `json:"model_type" binding:"omitempty,oneof=chat embedding rerank"`
	Params    *ModelParams      `json:"params" binding:"omitempty"`
}

// UpdateModelRequest 更新模型请求
type UpdateModelRequest struct {
	Name      string            `json:"name" binding:"omitempty,max=100"`
	Remark    string            `json:"remark" binding:"omitempty,max=255"`
	Protocol  string            `json:"protocol" binding:"omitempty,max=32"`
	BaseURL   string            `json:"base_url" binding:"omitempty,max=500"`
	APIKey    string            `json:"api_key" binding:"omitempty,max=500"`
	Headers   map[string]string `json:"headers" binding:"omitempty"`
	Timeout   int               `json:"timeout" binding:"omitempty,min=1000,max=300000"`
	Enabled   *bool             `json:"enabled" binding:"omitempty"`
	ModelType string            `json:"model_type" binding:"omitempty,oneof=chat embedding rerank"`
	Params    *ModelParams      `json:"params" binding:"omitempty"`
}

// ModelListRequest 模型列表请求
type ModelListRequest struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword   string `form:"keyword" binding:"omitempty"`
	ModelType string `form:"model_type" binding:"omitempty,oneof=chat embedding rerank"`
}

// FetchRemoteModelsRequest 远程拉取模型列表请求
type FetchRemoteModelsRequest struct {
	BaseURL  string `json:"base_url" binding:"required,max=500"`
	APIKey   string `json:"api_key" binding:"omitempty,max=500"`
	Protocol string `json:"protocol" binding:"omitempty,oneof=openai ollama"`
}

// ModelConnectionTestRequest 模型连接测试请求
type ModelConnectionTestRequest struct {
	ModelID   uint              `json:"model_id" binding:"omitempty,min=1"`
	Name      string            `json:"name" binding:"required,max=100"`
	Protocol  string            `json:"protocol" binding:"required,max=32"`
	BaseURL   string            `json:"base_url" binding:"required,max=500"`
	APIKey    string            `json:"api_key" binding:"omitempty,max=500"`
	Headers   map[string]string `json:"headers" binding:"omitempty"`
	Timeout   int               `json:"timeout" binding:"omitempty,min=1000,max=300000"`
	ModelType string            `json:"model_type" binding:"required,oneof=chat embedding rerank"`
}
