package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"Qavor/pkg/utils"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// 文档元数据中的键名，用于在 Eino 节点间传递分块来源信息。
const (
	MetaKeyKBID       = "kb_id"
	MetaKeyFileID     = "file_id"
	MetaKeyFilename   = "filename"
	MetaKeyChunkID    = "chunk_id"
	MetaKeyChunkIndex = "chunk_index"
	MetaKeyTokenCount = "token_count"
)

// DocumentTransformer 将 Markdown 文档稳定分块为 schema.Document 切片。
// 实现 Eino document.Transformer，输入输出均为 []*schema.Document。
type DocumentTransformer struct {
	splitter markdownSplitter
}

// markdownSplitter 抽象纯文本分块逻辑，便于测试替换。
type markdownSplitter interface {
	Split(markdown string) ([]string, error)
}

// NewDocumentTransformer 创建 Eino 文档分块 Transformer。
func NewDocumentTransformer(maxTokens, overlapTokens int) *DocumentTransformer {
	return &DocumentTransformer{splitter: NewChunker(maxTokens, overlapTokens)}
}

// Transform 实现 document.Transformer。
// 输入应为单个包含 Markdown 内容和来源元数据的文档；输出为按顺序分块后的文档切片。
// 继承 kb_id、file_id、filename，新增 chunk_index、token_count 和稳定 chunk_id。
func (t *DocumentTransformer) Transform(_ context.Context, src []*schema.Document, _ ...document.TransformerOption) ([]*schema.Document, error) {
	if t == nil || t.splitter == nil {
		return nil, errors.New("document transformer not configured")
	}
	if len(src) == 0 {
		return nil, errors.New("empty source documents")
	}
	source := src[0]
	if source == nil {
		return nil, errors.New("nil source document")
	}

	chunks, err := t.splitter.Split(source.Content)
	if err != nil {
		return nil, fmt.Errorf("split markdown: %w", err)
	}

	fileID := metaDataString(source, MetaKeyFileID)
	out := make([]*schema.Document, 0, len(chunks))
	for i, text := range chunks {
		tokenCount := utils.CountTokens(text)
		meta := copyMetaData(source.MetaData)
		meta[MetaKeyChunkIndex] = i
		meta[MetaKeyTokenCount] = tokenCount
		out = append(out, &schema.Document{
			ID:       buildChunkID(fileID, i, text),
			Content:  text,
			MetaData: meta,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("no valid chunks produced")
	}
	return out, nil
}

// copyMetaData 复制输入文档的元数据，避免修改原始 map。
func copyMetaData(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src)+2)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// metaDataString 安全读取元数据中的字符串值。
func metaDataString(doc *schema.Document, key string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	if v, ok := doc.MetaData[key].(string); ok {
		return v
	}
	return ""
}

// buildChunkID 基于 fileID + chunkIndex + contentHash 生成稳定的 ChunkID，重试不会重复。
func buildChunkID(fileID string, index int, content string) string {
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])
	return fmt.Sprintf("%s-%d-%s", fileID, index, hash[:16])
}
