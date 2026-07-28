package request

import "Qavor/internal/model/entity"

// CreateModelProviderRequest 创建模型供应商请求
type CreateModelProviderRequest struct {
	ProviderID              string           `json:"provider_id" binding:"required,max=100"`
	DisplayName             string           `json:"display_name" binding:"required,max=100"`
	ProviderType            string           `json:"provider_type" binding:"required,max=32"`
	DefaultProtocol         string           `json:"default_protocol" binding:"omitempty,max=64"`
	BaseURL                 string           `json:"base_url" binding:"required,max=500"`
	EmbeddingBaseURL        string           `json:"embedding_base_url" binding:"omitempty,max=500"`
	RerankBaseURL           string           `json:"rerank_base_url" binding:"omitempty,max=500"`
	ModelsEndpoint          string           `json:"models_endpoint" binding:"omitempty,max=200"`
	EmbeddingModelsEndpoint string           `json:"embedding_models_endpoint" binding:"omitempty,max=200"`
	RerankModelsEndpoint    string           `json:"rerank_models_endpoint" binding:"omitempty,max=200"`
	APIKeyEnv               string           `json:"api_key_env" binding:"omitempty,max=128"`
	APIKey                  string           `json:"api_key" binding:"omitempty,max=500"`
	Capabilities            entity.JSONArray `json:"capabilities" binding:"omitempty"`
	EnabledModels           entity.JSONArray `json:"enabled_models" binding:"omitempty"`
	HeadersJSON             entity.JSON      `json:"headers_json" binding:"omitempty"`
	ExtraJSON               entity.JSON      `json:"extra_json" binding:"omitempty"`
}

// UpdateModelProviderRequest 更新模型供应商请求
type UpdateModelProviderRequest struct {
	DisplayName             string           `json:"display_name" binding:"omitempty,max=100"`
	DefaultProtocol         string           `json:"default_protocol" binding:"omitempty,max=64"`
	BaseURL                 string           `json:"base_url" binding:"omitempty,max=500"`
	EmbeddingBaseURL        string           `json:"embedding_base_url" binding:"omitempty,max=500"`
	RerankBaseURL           string           `json:"rerank_base_url" binding:"omitempty,max=500"`
	ModelsEndpoint          string           `json:"models_endpoint" binding:"omitempty,max=200"`
	EmbeddingModelsEndpoint string           `json:"embedding_models_endpoint" binding:"omitempty,max=200"`
	RerankModelsEndpoint    string           `json:"rerank_models_endpoint" binding:"omitempty,max=200"`
	APIKeyEnv               string           `json:"api_key_env" binding:"omitempty,max=128"`
	APIKey                  string           `json:"api_key" binding:"omitempty,max=500"`
	Capabilities            entity.JSONArray `json:"capabilities" binding:"omitempty"`
	EnabledModels           entity.JSONArray `json:"enabled_models" binding:"omitempty"`
	HeadersJSON             entity.JSON      `json:"headers_json" binding:"omitempty"`
	ExtraJSON               entity.JSON      `json:"extra_json" binding:"omitempty"`
	IsEnabled               *bool            `json:"is_enabled" binding:"omitempty"`
}

// ModelProviderListRequest 模型供应商列表请求
type ModelProviderListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword" binding:"omitempty"`
}
