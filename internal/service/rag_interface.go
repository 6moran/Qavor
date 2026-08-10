package service

import "context"

// RAGService RAG 问答服务接口。
type RAGService interface {
	Retrieve(ctx context.Context, kbIDs []string, query string, topK int) (*RAGRetrieveResult, error)
	Answer(ctx context.Context, kbIDs []string, query string) (*RAGAnswerResult, error)
}

// RAGRetrieveResult RAG 检索服务返回结果。
type RAGRetrieveResult struct {
	QueryText string     `json:"query_text"`
	Chunks    []RAGChunk `json:"chunks"`
}

// RAGChunk 单条结构化检索结果。
type RAGChunk struct {
	KBID         string  `json:"kb_id"`
	ChunkID      string  `json:"chunk_id"`
	FileID       string  `json:"file_id"`
	Filename     string  `json:"filename"`
	DocumentName string  `json:"document_name,omitempty"`
	ResourceURI  string  `json:"resource_uri,omitempty"`
	Content      string  `json:"content"`
	Score        float64 `json:"score"`
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
