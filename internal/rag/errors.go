package rag

import (
	"context"
	"errors"
)

// DocumentIndexer 文档索引接口，供 Worker 注入并在解析完成后调用。
// 由 DocumentIndexPipeline 实现，Worker 调用方式不因 Eino 改造而扩散。
type DocumentIndexer interface {
	Index(ctx context.Context, in IndexInput) (*IndexOutput, error)
}

// AnswerChain 问答链路接口，供 Service 注入并映射业务错误码。
// 由 AnswerEngine 实现，保持 Service 调用方式不变。
type AnswerChain interface {
	Answer(ctx context.Context, in AnswerInput) (*AnswerOutput, error)
}

// RAG 链路业务错误。Service 通过 errors.Is 映射到 HTTP 错误码。
var (
	// ErrEmbeddingNotConfigured 表示 Embedding 缺失，索引和问答都不应继续。
	ErrEmbeddingNotConfigured = errors.New("Embedding 未配置")

	// ErrLLMNotConfigured 表示 ChatModel 缺失，问答不可用但索引仍可运行。
	ErrLLMNotConfigured = errors.New("LLM 未配置")

	// ErrEmbeddingUnavailable Embedding 调用失败。
	ErrEmbeddingUnavailable = errors.New("Embedding 服务不可用")

	// ErrRetrievalUnavailable 检索分支均不可用。
	ErrRetrievalUnavailable = errors.New("检索服务不可用")

	// ErrLLMUnavailable LLM 调用失败。
	ErrLLMUnavailable = errors.New("LLM 服务不可用")
)
