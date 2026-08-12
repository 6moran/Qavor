package response

// RAGSettingsResponse 是全局 RAG 设置响应。
type RAGSettingsResponse struct {
	RerankModelID   *uint  `json:"rerank_model_id"`
	RerankModelName string `json:"rerank_model_name"`
}
