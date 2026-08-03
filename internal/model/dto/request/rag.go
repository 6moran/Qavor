package request

// RAGAnswerRequest 问答接口请求。
type RAGAnswerRequest struct {
	KnowledgeBaseIDs []string `json:"knowledge_base_ids" binding:"required,min=1,max=10"`
	Query            string   `json:"query" binding:"required"`
}
