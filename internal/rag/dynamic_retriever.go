package rag

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
// 支持多个知识库使用不同 Embedding 模型：按模型分组检索后合并结果，按相似度排序并限制 TopK。
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

	// 1. 按知识库解析 Embedding 模型，按模型分组。
	type kbGroup struct {
		embeddingID uint
		kbIDs       []string
	}
	groups := make(map[uint]*kbGroup)
	var groupOrder []uint

	for _, kbID := range pgOpts.KnowledgeBaseIDs {
		base, err := r.kbRepo.FindByKBID(kbID)
		if err != nil {
			return nil, fmt.Errorf("find knowledge base %s: %w", kbID, err)
		}
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

	// 2. 解析 TopK（用于最终合并结果后排序截断）。
	defaultTopK := r.defaultK
	commonOpts := retriever.GetCommonOptions(&retriever.Options{TopK: &defaultTopK}, opts...)
	topK := *commonOpts.TopK
	if topK <= 0 {
		topK = r.defaultK
	}

	// 3. 每组分别 Embed + 检索，合并结果。
	var allDocs []*schema.Document
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
		allDocs = append(allDocs, docs...)
	}

	// 4. 按相似度降序排序并截断 TopK。
	sort.Slice(allDocs, func(i, j int) bool {
		si, _ := allDocs[i].MetaData[MetaKeyScore].(float64)
		sj, _ := allDocs[j].MetaData[MetaKeyScore].(float64)
		return si > sj
	})
	if len(allDocs) > topK {
		allDocs = allDocs[:topK]
	}

	return allDocs, nil
}

var _ retriever.Retriever = (*DynamicRetriever)(nil)
