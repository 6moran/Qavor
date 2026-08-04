package rag

import (
	"context"
	"errors"
	"fmt"

	"Qavor/internal/repository"
	"Qavor/pkg/config"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
)

// ModelResolver 根据知识库绑定的模型 ID 创建 Eino 运行组件。
// 模型配置和密钥由模型管理模块负责，RAG 只依赖这个运行时接口。
type ModelResolver interface {
	ResolveEmbedding(ctx context.Context, modelID uint) (embedding.Embedder, error)
	ResolveChatModel(ctx context.Context, modelID uint) (model.BaseChatModel, error)
}

// DynamicDocumentIndexer 在每次文档索引时根据 KBID 解析知识库绑定的 Embedding 模型。
// 这样 Worker 不会持有来自配置文件的全局模型。
type DynamicDocumentIndexer struct {
	kbRepo      repository.KnowledgeBaseRepository
	resolver    ModelResolver
	chunkRepo   repository.KnowledgeChunkRepository
	chunkTokens int
	overlap     int
	batchSize   int
	dimension   int
}

// NewDynamicDocumentIndexer 创建按知识库解析模型的索引器。
func NewDynamicDocumentIndexer(kbRepo repository.KnowledgeBaseRepository, resolver ModelResolver, chunkRepo repository.KnowledgeChunkRepository, chunkTokens, overlap, batchSize, dimension int) *DynamicDocumentIndexer {
	return &DynamicDocumentIndexer{
		kbRepo:      kbRepo,
		resolver:    resolver,
		chunkRepo:   chunkRepo,
		chunkTokens: chunkTokens,
		overlap:     overlap,
		batchSize:   batchSize,
		dimension:   dimension,
	}
}

// Index 实现 DocumentIndexer。
func (i *DynamicDocumentIndexer) Index(ctx context.Context, in IndexInput) (*IndexOutput, error) {
	if i == nil || i.kbRepo == nil || i.resolver == nil {
		return nil, ErrEmbeddingNotConfigured
	}
	base, err := i.kbRepo.FindByKBID(in.KBID)
	if err != nil {
		return nil, fmt.Errorf("find knowledge base: %w", err)
	}
	if base == nil {
		return nil, errors.New("knowledge base not found")
	}
	if base.EmbeddingModelID == 0 {
		return nil, ErrEmbeddingNotConfigured
	}
	emb, err := i.resolver.ResolveEmbedding(ctx, base.EmbeddingModelID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEmbeddingUnavailable, err)
	}
	chunkTokens := in.ChunkTokens
	if chunkTokens <= 0 {
		chunkTokens = i.chunkTokens
	}
	overlap := in.OverlapTokens
	if overlap < 0 {
		overlap = i.overlap
	}
	transformer := NewDocumentTransformer(chunkTokens, overlap)
	indexer := NewPGVectorIndexer(emb, i.chunkRepo, i.batchSize, i.dimension)
	pipeline, err := NewDocumentIndexPipeline(transformer, indexer)
	if err != nil {
		return nil, err
	}
	return pipeline.Index(ctx, in)
}

// DynamicAnswerEngine 按知识库绑定解析 Embedding/Chat 模型，然后执行 Eino Answer Graph。
type DynamicAnswerEngine struct {
	kbRepo    repository.KnowledgeBaseRepository
	resolver  ModelResolver
	chunkRepo repository.KnowledgeChunkRepository
	cfg       config.RAGConfig
}

// NewDynamicAnswerEngine 创建按知识库解析模型的问答引擎。
func NewDynamicAnswerEngine(kbRepo repository.KnowledgeBaseRepository, resolver ModelResolver, chunkRepo repository.KnowledgeChunkRepository, cfg config.RAGConfig) *DynamicAnswerEngine {
	return &DynamicAnswerEngine{kbRepo: kbRepo, resolver: resolver, chunkRepo: chunkRepo, cfg: cfg}
}

// ErrEmbeddingModelMismatch 表示一次问答选择的知识库绑定了不同的 Embedding 模型。
var ErrEmbeddingModelMismatch = errors.New("knowledge bases use different embedding models")

// ErrChatModelMismatch 表示一次问答选择的知识库绑定了不同的 Chat 模型。
var ErrChatModelMismatch = errors.New("knowledge bases use different chat models")

// Answer 实现 AnswerChain。
func (e *DynamicAnswerEngine) Answer(ctx context.Context, in AnswerInput) (*AnswerOutput, error) {
	if e == nil || e.kbRepo == nil || e.resolver == nil {
		return nil, ErrEmbeddingNotConfigured
	}
	if len(in.KnowledgeBaseIDs) == 0 {
		return nil, errors.New("knowledge base ids are required")
	}
	var embeddingID, chatID uint
	for _, kbID := range in.KnowledgeBaseIDs {
		base, err := e.kbRepo.FindByKBID(kbID)
		if err != nil {
			return nil, fmt.Errorf("find knowledge base: %w", err)
		}
		if base == nil {
			return nil, errors.New("knowledge base not found")
		}
		if base.EmbeddingModelID == 0 {
			return nil, ErrEmbeddingNotConfigured
		}
		if base.ChatModelID == 0 {
			return nil, ErrLLMNotConfigured
		}
		if embeddingID == 0 {
			embeddingID = base.EmbeddingModelID
		} else if embeddingID != base.EmbeddingModelID {
			return nil, ErrEmbeddingModelMismatch
		}
		if chatID == 0 {
			chatID = base.ChatModelID
		} else if chatID != base.ChatModelID {
			return nil, ErrChatModelMismatch
		}
	}
	emb, err := e.resolver.ResolveEmbedding(ctx, embeddingID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEmbeddingUnavailable, err)
	}
	chat, err := e.resolver.ResolveChatModel(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLLMUnavailable, err)
	}
	retriever := NewPGVectorRetriever(emb, e.chunkRepo, e.cfg.VectorTopK)
	graph, err := NewAnswerGraph(retriever, NewRAGChatTemplate(), chat)
	if err != nil {
		return nil, err
	}
	return graph.Answer(ctx, in)
}

var _ DocumentIndexer = (*DynamicDocumentIndexer)(nil)
var _ AnswerChain = (*DynamicAnswerEngine)(nil)
