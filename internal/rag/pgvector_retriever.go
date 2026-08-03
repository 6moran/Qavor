package rag

import (
	"context"
	"errors"
	"fmt"
	"math"

	"Qavor/internal/repository"
	"Qavor/pkg/logger"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

// MetaKeyScore 检索结果相似度得分在 schema.Document.MetaData 中的键名。
const MetaKeyScore = "score"

// pgRetrieverOptions 是 PGVectorRetriever 的实现特定选项，承载 KB IDs。
// 通过 retriever.WrapImplSpecificOptFn 与标准 Option 一起传递，
// 不放入全局变量或 context value。
type pgRetrieverOptions struct {
	KnowledgeBaseIDs []string
}

// WithKnowledgeBaseIDs 设置检索的知识库范围。
// 返回的 retriever.Option 仅在 PGVectorRetriever.Retrieve 中被解析。
func WithKnowledgeBaseIDs(kbIDs []string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *pgRetrieverOptions) {
		o.KnowledgeBaseIDs = kbIDs
	})
}

// PGVectorRetriever 实现 Eino retriever.Retriever，基于 pgvector 进行余弦相似度检索。
// 查询向量化使用构造时注入的 Embedder，可被 retriever.WithEmbedding 覆盖。
type PGVectorRetriever struct {
	embedder embedding.Embedder
	repo     repository.KnowledgeChunkRepository
	topK     int
}

// NewPGVectorRetriever 创建 pgvector 检索器。
// embedder 作为默认 Embedding 客户端，可被 WithEmbedding 覆盖。
// topK > 0 时作为默认 TopK，可被 WithTopK 覆盖。
func NewPGVectorRetriever(embedder embedding.Embedder, repo repository.KnowledgeChunkRepository, topK int) *PGVectorRetriever {
	if topK <= 0 {
		topK = 5
	}
	return &PGVectorRetriever{
		embedder: embedder,
		repo:     repo,
		topK:     topK,
	}
}

// Retrieve 实现 retriever.Retriever。
// 标准选项 TopK、ScoreThreshold、Embedding 通过 GetCommonOptions 解析；
// KB IDs 通过 GetImplSpecificOptions 解析。
// Repository SQL 同时过滤 kb_id IN (...) 和文件 ready 状态，按余弦相似度排序并限制 TopK。
// 返回 schema.Document，将 chunk_id、file_id、filename、kb_id、score 放入 MetaData。
func (r *PGVectorRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if r == nil {
		return nil, errors.New("retriever not configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.repo == nil {
		return nil, errors.New("retriever repository not configured")
	}
	if query == "" {
		return nil, errors.New("empty query")
	}

	// 1. 解析标准选项：TopK、ScoreThreshold、Embedding。
	defaultTopK := r.topK
	options := retriever.GetCommonOptions(&retriever.Options{TopK: &defaultTopK}, opts...)

	// 2. 解析实现特定选项：KB IDs。
	pgOpts := retriever.GetImplSpecificOptions(&pgRetrieverOptions{}, opts...)
	if len(pgOpts.KnowledgeBaseIDs) == 0 {
		return nil, errors.New("knowledge base ids are required")
	}

	// 3. 选择 Embedder：调用方显式注入优先，否则使用构造时的默认值。
	emb := options.Embedding
	if emb == nil {
		emb = r.embedder
	}
	if emb == nil {
		return nil, ErrEmbeddingNotConfigured
	}

	// 4. 查询向量化。
	vectors, err := emb.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, ErrEmbeddingUnavailable
	}
	if err := validateVector(vectors[0]); err != nil {
		return nil, fmt.Errorf("validate query vector: %w", err)
	}
	queryVector := pgvector.NewVector(toFloat32(vectors[0]))

	// 5. 向量检索：Repository 已过滤 kb_id 和 ready 文件状态，按余弦相似度排序并限制 TopK。
	topK := *options.TopK
	if topK <= 0 {
		topK = r.topK
	}
	rows, err := r.repo.FindNearestByKBIDs(ctx, pgOpts.KnowledgeBaseIDs, queryVector, topK)
	if err != nil {
		return nil, fmt.Errorf("find nearest chunks: %w", err)
	}

	// 6. 映射为 schema.Document，过滤低于阈值的结果。
	threshold := 0.0
	if options.ScoreThreshold != nil {
		threshold = *options.ScoreThreshold
	}
	docs := make([]*schema.Document, 0, len(rows))
	for _, row := range rows {
		if math.IsNaN(row.Score) || row.Score < threshold {
			continue
		}
		docs = append(docs, &schema.Document{
			ID:      row.ChunkID,
			Content: row.Content,
			MetaData: map[string]any{
				MetaKeyChunkID:  row.ChunkID,
				MetaKeyFileID:   row.FileID,
				MetaKeyFilename: row.Filename,
				MetaKeyKBID:     row.KBID,
				MetaKeyScore:    row.Score,
			},
		})
	}

	if logger.Initialized() {
		logger.Debug("向量检索完成",
			zap.Int("kb_count", len(pgOpts.KnowledgeBaseIDs)),
			zap.Int("top_k", topK),
			zap.Int("hits", len(docs)),
		)
	}
	return docs, nil
}
