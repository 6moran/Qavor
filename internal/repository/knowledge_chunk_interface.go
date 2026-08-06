package repository

import (
	"context"

	"Qavor/internal/model/entity"

	"github.com/pgvector/pgvector-go"
)

// KnowledgeChunkRepository 知识分块数据访问接口。
// 负责在事务中替换指定文件的所有分块，避免索引失败时留下部分数据。
type KnowledgeChunkRepository interface {
	// FindByFileID 按分块序号获取指定文件的全部分块。
	FindByFileID(ctx context.Context, kbID, fileID string) ([]*entity.KnowledgeChunk, error)
	// ReplaceByFileID 事务内：删除 fileID 下已有分块，再批量写入新分块。
	ReplaceByFileID(ctx context.Context, kbID, fileID string, chunks []*entity.KnowledgeChunk) error
	// FindNearestByKBIDs 按知识库列表向量检索 TopK，只查询 ready 状态的文件。
	FindNearestByKBIDs(ctx context.Context, kbIDs []string, queryVector pgvector.Vector, limit int) ([]ChunkWithFile, error)
}

// ChunkWithFile 携带文件名的分块结果，用于向量检索直接构造引用。
type ChunkWithFile struct {
	ChunkID  string  `gorm:"column:chunk_id"`
	KBID     string  `gorm:"column:kb_id"`
	FileID   string  `gorm:"column:file_id"`
	Filename string  `gorm:"column:filename"`
	Content  string  `gorm:"column:content"`
	Score    float64 `gorm:"column:score"`
}
