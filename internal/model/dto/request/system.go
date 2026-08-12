package request

// UpdateRAGSettingsRequest 更新全局 RAG 设置请求。
type UpdateRAGSettingsRequest struct {
	RerankModelID *uint `json:"rerank_model_id" binding:"omitempty,min=1"`
}

// UpdateSystemConfigRequest 更新单个系统配置项请求。
type UpdateSystemConfigRequest struct {
	Key   string `json:"key" binding:"required"`
	Value any    `json:"value"`
}

// UpdateConfigOptionRequest 更新单个配置项（OCR 服务配置等）请求。
type UpdateConfigOptionRequest struct {
	Value map[string]string `json:"value"`
}
