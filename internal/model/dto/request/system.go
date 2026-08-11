package request

// UpdateRAGSettingsRequest 更新全局 RAG 设置请求。
type UpdateRAGSettingsRequest struct {
	RerankModelID *uint `json:"rerank_model_id" binding:"omitempty,min=1"`
}
