package rag

import (
	"context"
	"errors"
	"fmt"
	"math"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/logger"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

// PGVectorIndexer 实现 Eino indexer.Indexer，负责批量 Embedding、向量校验和调用事务 Repository。
type PGVectorIndexer struct {
	embedder  embedding.Embedder
	repo      repository.KnowledgeChunkRepository
	batchSize int
	dimension int
}

// NewPGVectorIndexer 创建 pgvector 索引器。
// embedder 为 Eino embedding.Embedder；dimension > 0 时校验返回向量维度。
func NewPGVectorIndexer(embedder embedding.Embedder, repo repository.KnowledgeChunkRepository, batchSize, dimension int) *PGVectorIndexer {
	if batchSize <= 0 {
		batchSize = 32
	}
	return &PGVectorIndexer{
		embedder:  embedder,
		repo:      repo,
		batchSize: batchSize,
		dimension: dimension,
	}
}

// Store 实现 indexer.Indexer.Store。
// 先生成全部向量，校验通过后一次事务替换，保证重试幂等且不会先删后败。
// 返回稳定 Chunk ID 列表。
func (ix *PGVectorIndexer) Store(ctx context.Context, docs []*schema.Document, _ ...indexer.Option) ([]string, error) {
	if ix == nil || ix.embedder == nil {
		return nil, ErrEmbeddingNotConfigured
	}
	if ix.repo == nil {
		return nil, errors.New("indexer repository not configured")
	}
	if len(docs) == 0 {
		return nil, errors.New("no documents to index")
	}

	kbID, fileID := extractDocOrigin(docs[0])
	if kbID == "" || fileID == "" {
		return nil, fmt.Errorf("missing kb_id or file_id in document metadata")
	}

	// 1. 批量 Embedding
	contents := make([]string, len(docs))
	for i, d := range docs {
		contents[i] = d.Content
	}
	vectors, err := ix.embedBatched(ctx, contents)
	if err != nil {
		return nil, fmt.Errorf("embed chunks: %w", err)
	}

	// 2. 构造实体并组装稳定 ID
	chunks := make([]*entity.KnowledgeChunk, 0, len(docs))
	ids := make([]string, 0, len(docs))
	for i, doc := range docs {
		if i < len(vectors) && vectors[i] != nil {
			if err := validateVector(vectors[i]); err != nil {
				return nil, fmt.Errorf("validate vector %d: %w", i, err)
			}
		}
		chunk := &entity.KnowledgeChunk{
			ChunkID:    doc.ID,
			FileID:     fileID,
			KBID:       kbID,
			ChunkIndex: metaDataInt(doc, MetaKeyChunkIndex, i),
			Content:    doc.Content,
			TokenCount: metaDataInt(doc, MetaKeyTokenCount, 0),
		}
		if i < len(vectors) && vectors[i] != nil {
			chunk.Embedding = pgvector.NewVector(toFloat32(vectors[i]))
		}
		chunks = append(chunks, chunk)
		ids = append(ids, doc.ID)
	}

	// 3. 事务替换：先生成全部向量再写库，任一批次失败都不写。
	if err := ix.repo.ReplaceByFileID(ctx, kbID, fileID, chunks); err != nil {
		return nil, fmt.Errorf("replace chunks: %w", err)
	}

	if logger.Initialized() {
		logger.Debug("文档索引完成",
			zap.String("kb_id", kbID),
			zap.String("file_id", fileID),
			zap.Int("chunks", len(chunks)),
		)
	}
	return ids, nil
}

// embedBatched 以 batchSize 为粒度调用 Embedding，防止单次请求过大。
func (ix *PGVectorIndexer) embedBatched(ctx context.Context, chunks []string) ([][]float64, error) {
	vectors := make([][]float64, len(chunks))
	for i := 0; i < len(chunks); i += ix.batchSize {
		end := i + ix.batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		part := chunks[i:end]
		got, err := ix.embedder.EmbedStrings(ctx, part)
		if err != nil {
			return nil, err
		}
		if len(got) != len(part) {
			return nil, fmt.Errorf("embedding size mismatch: want %d got %d", len(part), len(got))
		}
		for j, v := range got {
			if err := validateVectorDims(v, ix.dimension); err != nil {
				return nil, fmt.Errorf("vector %d: %w", i+j, err)
			}
			vectors[i+j] = v
		}
	}
	return vectors, nil
}

// validateVector 拒绝 NaN / Inf 向量。
func validateVector(v []float64) error {
	if len(v) == 0 {
		return errors.New("empty embedding vector")
	}
	for _, f := range v {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return errors.New("embedding contains NaN or Inf")
		}
	}
	return nil
}

// validateVectorDims 在维度有要求时校验维度一致性。
func validateVectorDims(v []float64, expect int) error {
	if err := validateVector(v); err != nil {
		return err
	}
	if expect > 0 && len(v) != expect {
		return fmt.Errorf("dimension mismatch: want %d got %d", expect, len(v))
	}
	return nil
}

// extractDocOrigin 从文档元数据中提取 kb_id 和 file_id。
func extractDocOrigin(doc *schema.Document) (kbID, fileID string) {
	if doc == nil {
		return "", ""
	}
	return metaDataString(doc, MetaKeyKBID), metaDataString(doc, MetaKeyFileID)
}

// metaDataInt 安全读取元数据中的整数值，缺失时返回 fallback。
func metaDataInt(doc *schema.Document, key string, fallback int) int {
	if doc == nil || doc.MetaData == nil {
		return fallback
	}
	switch v := doc.MetaData[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return fallback
	}
}

// metaDataFloat64 安全读取元数据中的 float64 值，缺失时返回 fallback。
func metaDataFloat64(doc *schema.Document, key string, fallback float64) float64 {
	if doc == nil || doc.MetaData == nil {
		return fallback
	}
	switch v := doc.MetaData[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return fallback
	}
}

// toFloat32 pgvector-go 的 Vector 需要 float32 切片。
func toFloat32(v []float64) []float32 {
	out := make([]float32, len(v))
	for i, f := range v {
		out[i] = float32(f)
	}
	return out
}
