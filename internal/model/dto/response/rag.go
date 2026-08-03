package response

import "Qavor/internal/rag"

// RAGAnswerResponse 问答接口响应。
type RAGAnswerResponse struct {
	Answer    string         `json:"answer"`
	Citations []rag.Citation `json:"citations"`
}
