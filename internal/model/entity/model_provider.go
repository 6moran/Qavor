package entity

// ModelProvider 模型供应商实体
type ModelProvider struct {
	BaseEntity
	ProviderID              string    `gorm:"type:varchar(100);uniqueIndex;not null;comment:供应商稳定标识" json:"provider_id"`
	DisplayName             string    `gorm:"type:varchar(100);not null;comment:展示名称" json:"display_name"`
	ProviderType            string    `gorm:"type:varchar(32);not null;default:openai;comment:供应商适配类型" json:"provider_type"`
	DefaultProtocol         string    `gorm:"type:varchar(64);comment:默认协议，如openai_compatible" json:"default_protocol,omitempty"`
	BaseURL                 string    `gorm:"type:varchar(500);not null;comment:API基础URL" json:"base_url"`
	EmbeddingBaseURL        string    `gorm:"type:varchar(500);comment:Embedding模型请求基础URL" json:"embedding_base_url,omitempty"`
	RerankBaseURL           string    `gorm:"type:varchar(500);comment:Rerank模型请求基础URL" json:"rerank_base_url,omitempty"`
	ModelsEndpoint          string    `gorm:"type:varchar(200);comment:聊天/通用模型列表端点" json:"models_endpoint,omitempty"`
	EmbeddingModelsEndpoint string    `gorm:"type:varchar(200);comment:Embedding模型列表端点" json:"embedding_models_endpoint,omitempty"`
	RerankModelsEndpoint    string    `gorm:"type:varchar(200);comment:Rerank模型列表端点" json:"rerank_models_endpoint,omitempty"`
	APIKeyEnv               string    `gorm:"type:varchar(128);comment:API Key环境变量名" json:"api_key_env,omitempty"`
	APIKey                  string    `gorm:"type:varchar(500);comment:直接配置的API Key" json:"api_key,omitempty"`
	Capabilities            JSONArray `gorm:"type:json;not null;default:'[]';comment:支持能力：chat/embedding/rerank" json:"capabilities"`
	EnabledModels           JSONArray `gorm:"type:json;not null;default:'[]';comment:已启用模型配置" json:"enabled_models"`
	HeadersJSON             JSON      `gorm:"type:json;default:{};comment:额外请求头" json:"headers_json,omitempty"`
	ExtraJSON               JSON      `gorm:"type:json;default:{};comment:扩展配置" json:"extra_json,omitempty"`
	IsEnabled               bool      `gorm:"not null;default:true;index;comment:是否启用" json:"is_enabled"`
	IsBuiltin               bool      `gorm:"not null;default:false;comment:是否内置" json:"is_builtin"`
}

// TableName 指定表名
func (ModelProvider) TableName() string {
	return "model_providers"
}
