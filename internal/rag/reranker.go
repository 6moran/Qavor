package rag

import "context"

// RerankDocument 是发送给重排模型的候选文档。
type RerankDocument struct {
	ID      string
	Content string
}

// RerankResult 表示模型返回的候选索引与相关性分数。
type RerankResult struct {
	Index int
	Score float64
}

// Reranker 定义重排模型的最小运行时接口。
type Reranker interface {
	Rerank(ctx context.Context, query string, documents []RerankDocument, topN int) ([]RerankResult, error)
}
