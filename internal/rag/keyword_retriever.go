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

// KeywordRetriever 使用 PostgreSQL pg_trgm 执行字符级关键词召回。
type KeywordRetriever struct {
	repo     repository.KnowledgeChunkRepository
	defaultK int
}

// NewKeywordRetriever 创建关键词检索器。
func NewKeywordRetriever(repo repository.KnowledgeChunkRepository, defaultK int) *KeywordRetriever {
	if defaultK <= 0 {
		defaultK = 5
	}
	return &KeywordRetriever{repo: repo, defaultK: defaultK}
}

// Retrieve 归一化查询，并在指定知识库范围内检索已入库分块。
func (r *KeywordRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if r == nil || r.repo == nil {
		return nil, errors.New("关键词检索器未配置")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalized := normalizeKeywordQuery(query)
	if normalized == "" {
		return nil, errors.New("查询不能为空")
	}
	implementationOptions := retriever.GetImplSpecificOptions(&pgRetrieverOptions{}, opts...)
	if len(implementationOptions.KnowledgeBaseIDs) == 0 {
		return nil, errors.New("知识库 ID 不能为空")
	}
	defaultTopK := r.defaultK
	commonOptions := retriever.GetCommonOptions(&retriever.Options{TopK: &defaultTopK}, opts...)
	topK := defaultTopK
	if commonOptions.TopK != nil && *commonOptions.TopK > 0 {
		topK = *commonOptions.TopK
	}
	rows, err := r.repo.FindKeywordByKBIDs(ctx, implementationOptions.KnowledgeBaseIDs, normalized, topK)
	if err != nil {
		return nil, fmt.Errorf("%w: 关键词分块查询失败: %w", ErrRetrievalUnavailable, err)
	}
	documents := make([]*schema.Document, 0, len(rows))
	for _, row := range rows {
		documents = append(documents, &schema.Document{
			ID:      row.ChunkID,
			Content: row.Content,
			MetaData: map[string]any{
				MetaKeyChunkID:  row.ChunkID,
				MetaKeyKBID:     row.KBID,
				MetaKeyFileID:   row.FileID,
				MetaKeyFilename: row.Filename,
				MetaKeyScore:    row.Score,
			},
		})
	}
	return documents, nil
}

func normalizeKeywordQuery(query string) string {
	return strings.ToLower(strings.Join(strings.Fields(query), " "))
}

var _ retriever.Retriever = (*KeywordRetriever)(nil)
