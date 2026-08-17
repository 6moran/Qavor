package repository

import (
	"context"

	"Qavor/internal/model/entity"
)

// KnowledgeFileRepository 知识文件数据访问接口，定义持久化操作
type KnowledgeFileRepository interface {
	// Create 创建文件记录
	Create(file *entity.KnowledgeFile) error
	// FindByKBIDAndFileID 根据知识库ID和文件ID查询
	FindByKBIDAndFileID(kbID, fileID string) (*entity.KnowledgeFile, error)
	// ListByKBID 分页查询知识库下的文件列表
	ListByKBID(kbID string, offset, limit int, parentID, pathPrefix string, recursive bool, status string) ([]*entity.KnowledgeFile, int64, error)
	// ListAllByKBID 返回知识库下的全部文件记录，用于删除知识库时清理对象存储。
	ListAllByKBID(ctx context.Context, kbID string) ([]*entity.KnowledgeFile, error)
	// SearchByKBID 按文件名、原始文件名或路径检索知识库文件。
	SearchByKBID(kbID, query string, offset, limit int) ([]*entity.KnowledgeFile, int64, error)
	// DeleteByKBIDAndFileID 根据知识库ID和文件ID删除
	DeleteByKBIDAndFileID(kbID, fileID string) error
	// DeleteWithChunks 在一个数据库事务中删除文件的所有分块和文件记录。
	DeleteWithChunks(ctx context.Context, kbID, fileID string) error
	UpdateProcessingResult(kbID, fileID, status, markdownFile, errorMessage string) error
	// TransitionStatus 比较并设置文件状态，从允许的状态之一转换到目标状态。
	// 当恰好更新一行时返回 true。可选的 updates 映射在同一语句中应用。
	TransitionStatus(ctx context.Context, kbID, fileID string, from []string, to string, updates map[string]any) (bool, error)
	// ListByKBIDAndStatuses 返回指定知识库中匹配任一状态的文件，最多返回 limit 条。
	ListByKBIDAndStatuses(ctx context.Context, kbID string, statuses []string, limit int) ([]*entity.KnowledgeFile, error)
}
