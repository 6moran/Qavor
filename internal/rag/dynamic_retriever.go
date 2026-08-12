package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"Qavor/internal/model/entity"
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

// Retrieve 实现 Eino retriever.Retriever，并为旧的纯向量调用方顺序展开各模型组。
func (r *DynamicRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	lists, err := r.RetrieveRanked(ctx, query, opts...)
	if err != nil {
		return nil, err
	}
	documents := make([]*schema.Document, 0)
	for _, list := range lists {
		documents = append(documents, list...)
	}
	defaultTopK := r.defaultK
	commonOptions := retriever.GetCommonOptions(&retriever.Options{TopK: &defaultTopK}, opts...)
	topK := defaultTopK
	if commonOptions.TopK != nil && *commonOptions.TopK > 0 {
		topK = *commonOptions.TopK
	}
	if len(documents) > topK {
		documents = documents[:topK]
	}
	return documents, nil
}

// RetrieveRanked 为每个 Embedding 模型组返回独立的仓储排名列表。
func (r *DynamicRetriever) RetrieveRanked(ctx context.Context, query string, opts ...retriever.Option) ([][]*schema.Document, error) {
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

	// 1. 按知识库解析 Embedding 模型，按模型分组（一次批量查询）。
	bases, err := r.kbRepo.FindByKBIDs(pgOpts.KnowledgeBaseIDs)
	if err != nil {
		return nil, fmt.Errorf("find knowledge bases: %w", err)
	}
	byID := make(map[string]*entity.KnowledgeBase, len(bases))
	for _, base := range bases {
		byID[base.KBID] = base
	}
	type kbGroup struct {
		embeddingID uint
		kbIDs       []string
	}
	groups := make(map[uint]*kbGroup)
	var groupOrder []uint

	for _, kbID := range pgOpts.KnowledgeBaseIDs {
		base := byID[kbID]
		if base == nil {
			return nil, fmt.Errorf("knowledge base %s not found", kbID)
		}
		if base.EmbeddingModelID == 0 {
			return nil, fmt.Errorf("%w: knowledge base %s has no embedding model", ErrEmbeddingNotConfigured, kbID)
		}
		g, ok := groups[base.EmbeddingModelID]
		if !ok {
			g = &kbGroup{embeddingID: base.EmbeddingModelID}
			groups[base.EmbeddingModelID] = g
			groupOrder = append(groupOrder, base.EmbeddingModelID)
		}
		g.kbIDs = append(g.kbIDs, kbID)
	}

	// 2. 每组分别 Embed + 检索，并保持仓储返回顺序。
	lists := make([][]*schema.Document, 0, len(groupOrder))
	for _, embID := range groupOrder {
		g := groups[embID]
		emb, err := r.resolver.ResolveEmbedding(ctx, g.embeddingID)
		if err != nil {
			return nil, fmt.Errorf("%w: resolve embedding model %d: %w", ErrEmbeddingUnavailable, g.embeddingID, err)
		}
		// 覆盖 KB IDs 为当前分组
		groupOpts := make([]retriever.Option, 0, len(opts)+1)
		groupOpts = append(groupOpts, opts...)
		groupOpts = append(groupOpts, WithKnowledgeBaseIDs(g.kbIDs))
		docs, err := NewPGVectorRetriever(emb, r.chunkRepo, r.defaultK).Retrieve(ctx, query, groupOpts...)
		if err != nil {
			return nil, err
		}
		lists = append(lists, docs)
	}
	return lists, nil
}

// contentDigest 计算片段内容的 SHA-256 摘要，用于检索结果去重。
func contentDigest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

var _ retriever.Retriever = (*DynamicRetriever)(nil)
