package repository

import (
	"context"

	"Qavor/internal/model/entity"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

// knowledgeChunkRepository 知识分块数据访问实现。
type knowledgeChunkRepository struct {
	db *gorm.DB
}

// NewKnowledgeChunkRepository 创建知识分块仓储。
func NewKnowledgeChunkRepository(db *gorm.DB) KnowledgeChunkRepository {
	return &knowledgeChunkRepository{db: db}
}

// ReplaceByFileID 事务内替换分块：先删除再批量写入；任一失败都回滚。
func (r *knowledgeChunkRepository) ReplaceByFileID(ctx context.Context, kbID, fileID string, chunks []*entity.KnowledgeChunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("kb_id = ? AND file_id = ?", kbID, fileID).
			Delete(&entity.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if len(chunks) == 0 {
			return nil
		}
		return tx.Create(chunks).Error
	})
}

// FindNearestByKBIDs 向量检索 TopK，只查询 ready 状态的文件。
func (r *knowledgeChunkRepository) FindNearestByKBIDs(ctx context.Context, kbIDs []string, queryVector pgvector.Vector, limit int) ([]ChunkWithFile, error) {
	if len(kbIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	var rows []ChunkWithFile
	// pgvector 的 <=> 操作符不能通过 GORM 的占位符正常转义；使用原生 SQL 组装，
	// 保留对 kbIDs / limit 的参数绑定，避免 SQL 注入。
	sql := `SELECT c.chunk_id, c.kb_id, c.file_id, f.filename, c.content,
		1 - (c.embedding <=> $1) AS score
		FROM knowledge_chunks c
		JOIN knowledge_files f ON f.file_id = c.file_id
		WHERE c.kb_id = ANY($2)
		  AND f.status = 'ready'
		ORDER BY c.embedding <=> $1
		LIMIT $3`
	err := r.db.WithContext(ctx).Raw(sql, queryVector, kbIDs, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
