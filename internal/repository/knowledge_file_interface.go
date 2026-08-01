package repository

import "Qavor/internal/model/entity"

// KnowledgeFileRepository 知识文件数据访问接口，定义持久化操作
type KnowledgeFileRepository interface {
	// Create 创建文件记录
	Create(file *entity.KnowledgeFile) error
	// FindByKBIDAndFileID 根据知识库ID和文件ID查询
	FindByKBIDAndFileID(kbID, fileID string) (*entity.KnowledgeFile, error)
	// ListByKBID 分页查询知识库下的文件列表
	ListByKBID(kbID string, offset, limit int, parentID, pathPrefix string, recursive bool, status string) ([]*entity.KnowledgeFile, int64, error)
	// SearchByKBID 按文件名、原始文件名或路径检索知识库文件。
	SearchByKBID(kbID, query string, offset, limit int) ([]*entity.KnowledgeFile, int64, error)
	// DeleteByKBIDAndFileID 根据知识库ID和文件ID删除
	DeleteByKBIDAndFileID(kbID, fileID string) error
	UpdateProcessingResult(kbID, fileID, status, markdownFile, errorMessage string) error
}
