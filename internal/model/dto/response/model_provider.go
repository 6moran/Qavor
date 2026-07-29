package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// ModelProviderResponse 模型供应商响应
type ModelProviderResponse struct {
	ID                      uint             `json:"id"`
	ProviderID              string           `json:"provider_id"`
	DisplayName             string           `json:"display_name"`
	ProviderType            string           `json:"provider_type"`
	DefaultProtocol         string           `json:"default_protocol,omitempty"`
	BaseURL                 string           `json:"base_url"`
	EmbeddingBaseURL        string           `json:"embedding_base_url,omitempty"`
	RerankBaseURL           string           `json:"rerank_base_url,omitempty"`
	ModelsEndpoint          string           `json:"models_endpoint,omitempty"`
	EmbeddingModelsEndpoint string           `json:"embedding_models_endpoint,omitempty"`
	RerankModelsEndpoint    string           `json:"rerank_models_endpoint,omitempty"`
	APIKeyEnv               string           `json:"api_key_env,omitempty"`
	Capabilities            entity.JSONArray `json:"capabilities"`
	EnabledModels           entity.JSONArray `json:"enabled_models"`
	HeadersJSON             entity.JSON      `json:"headers_json,omitempty"`
	ExtraJSON               entity.JSON      `json:"extra_json,omitempty"`
	IsEnabled               bool             `json:"is_enabled"`
	IsBuiltin               bool             `json:"is_builtin"`
	CreatedAt               time.Time        `json:"created_at"`
	UpdatedAt               time.Time        `json:"updated_at"`
}

// ModelProviderListResponse 模型供应商列表响应
type ModelProviderListResponse struct {
	Total int64                   `json:"total"`
	Items []ModelProviderResponse `json:"items"`
}
