package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"Qavor/internal/repository"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// DynamicRetriever 根据本次请求的知识库配置解析 Embedding 模型并执行向量检索。
type DynamicRetriever struct {
	kbRepo    repository.KnowledgeBaseRepository
	resolver  ModelResolver
	chunkRepo repository.KnowledgeChunkRepository
	defaultK  int
}

// NewDynamicRetriever 创建按知识库动态解析 Embedding 模型的检索器。
func NewDynamicRetriever(
	kbRepo repository.KnowledgeBaseRepository,
	resolver ModelResolver,
	chunkRepo repository.KnowledgeChunkRepository,
	defaultK int,
) *DynamicRetriever {
	if defaultK <= 0 {
		defaultK = 5
	}
	return &DynamicRetriever{
		kbRepo:    kbRepo,
		resolver:  resolver,
		chunkRepo: chunkRepo,
		defaultK:  defaultK,
	}
}

// Retrieve 实现 Eino retriever.Retriever。
func (r *DynamicRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if r == nil || r.kbRepo == nil || r.resolver == nil || r.chunkRepo == nil {
		return nil, ErrEmbeddingNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("empty query")
	}

	pgOpts := retriever.GetImplSpecificOptions(&pgRetrieverOptions{}, opts...)
	if len(pgOpts.KnowledgeBaseIDs) == 0 {
		return nil, errors.New("knowledge base ids are required")
	}

	var embeddingID uint
	for _, kbID := range pgOpts.KnowledgeBaseIDs {
		base, err := r.kbRepo.FindByKBID(kbID)
		if err != nil {
			return nil, fmt.Errorf("find knowledge base: %w", err)
		}
		if base == nil {
			return nil, errors.New("knowledge base not found")
		}
		if base.EmbeddingModelID == 0 {
			return nil, ErrEmbeddingNotConfigured
		}
		if embeddingID == 0 {
			embeddingID = base.EmbeddingModelID
		} else if embeddingID != base.EmbeddingModelID {
			return nil, ErrEmbeddingModelMismatch
		}
	}

	emb, err := r.resolver.ResolveEmbedding(ctx, embeddingID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEmbeddingUnavailable, err)
	}

	return NewPGVectorRetriever(emb, r.chunkRepo, r.defaultK).Retrieve(ctx, query, opts...)
}

var _ retriever.Retriever = (*DynamicRetriever)(nil)
