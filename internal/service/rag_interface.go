package service

import "context"

// RAGService RAG 问答服务接口。
type RAGService interface {
	Answer(ctx context.Context, kbIDs []string, query string) (*RAGAnswerResult, error)
}

// RAGAnswerResult RAG 问答服务返回结果。
type RAGAnswerResult struct {
	Answer    string        `json:"answer"`
	Citations []RAGCitation `json:"citations"`
}

// RAGCitation 单条引用。
type RAGCitation struct {
	Index    int     `json:"index"`
	ChunkID  string  `json:"chunk_id"`
	FileID   string  `json:"file_id"`
	Filename string  `json:"filename"`
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
}
