package service

import "context"

// RAGService RAG 问答服务接口。
type RAGService interface {
	Retrieve(ctx context.Context, kbIDs []string, query string, topK int) (*RAGRetrieveResult, error)
	Answer(ctx context.Context, kbIDs []string, query string) (*RAGAnswerResult, error)
	// RetrieveTest 按单次请求的检索参数执行测试检索，不修改知识库配置。
	RetrieveTest(ctx context.Context, kbIDs []string, query string, cfg *RetrievalTestConfig) (*RAGRetrieveResult, error)
}

// RetrievalTestConfig 检索测试的单次参数覆盖。nil 字段表示沿用系统默认配置。
type RetrievalTestConfig struct {
	TopK           *int     // 最终返回条数，>0 时生效
	VectorTopK     *int     // 向量召回窗口
	KeywordTopK    *int     // 关键词召回窗口
	FusedTopK      *int     // RRF 融合窗口
	RerankTopK     *int     // 重排后返回条数
	RRFK           *int     // RRF 平滑参数
	ScoreThreshold *float64 // 相似度阈值过滤
}

// RAGRetrieveResult RAG 检索服务返回结果。
type RAGRetrieveResult struct {
	QueryText string     `json:"query_text"`
	Chunks    []RAGChunk `json:"chunks"`
}

// RAGChunk 单条结构化检索结果。
type RAGChunk struct {
	KBID         string   `json:"kb_id"`
	KBName       string   `json:"kb_name,omitempty"`
	ChunkID      string   `json:"chunk_id"`
	FileID       string   `json:"file_id"`
	Filename     string   `json:"filename"`
	DocumentName string   `json:"document_name,omitempty"`
	ResourceURI  string   `json:"resource_uri,omitempty"`
	Content      string   `json:"content"`
	Score        float64  `json:"score"`
	VectorScore  *float64 `json:"vector_score,omitempty"`
	KeywordScore *float64 `json:"keyword_score,omitempty"`
	RRFScore     *float64 `json:"rrf_score,omitempty"`
	RerankScore  *float64 `json:"rerank_score,omitempty"`
	MatchedBy    []string `json:"matched_by,omitempty"`
}

// RAGAnswerResult RAG 问答服务返回结果。
type RAGAnswerResult struct {
	Answer    string        `json:"answer"`
	Citations []RAGCitation `json:"citations"`
}

// RAGCitation 单条引用。
type RAGCitation struct {
	Index        int      `json:"index"`
	ChunkID      string   `json:"chunk_id"`
	FileID       string   `json:"file_id"`
	Filename     string   `json:"filename"`
	Content      string   `json:"content"`
	Score        float64  `json:"score"`
	VectorScore  *float64 `json:"vector_score,omitempty"`
	KeywordScore *float64 `json:"keyword_score,omitempty"`
	RRFScore     *float64 `json:"rrf_score,omitempty"`
	RerankScore  *float64 `json:"rerank_score,omitempty"`
	MatchedBy    []string `json:"matched_by,omitempty"`
}
