package repository

import "Qavor/internal/model/entity"

// KnowledgeBaseStats 知识库统计信息
type KnowledgeBaseStats struct {
	FileCount         int64 // 文件总数
	ChunkCount        int64 // 分块总数
	TokenCount        int64 // Token 总数
	TotalSize         int64 // 文件总大小（字节）
	ProcessingCount   int   // 处理中的文件数
	PendingParseCount int   // 待解析文件数
	PendingIndexCount int   // 待入库文件数
}

// KnowledgeBaseRepository 知识库数据访问接口，定义持久化操作
type KnowledgeBaseRepository interface {
	// Create 创建知识库记录
	Create(base *entity.KnowledgeBase) error
	// FindByKBID 根据知识库ID查询
	FindByKBID(kbID string) (*entity.KnowledgeBase, error)
	// List 分页查询知识库列表
	List(offset, limit int, keyword string) ([]*entity.KnowledgeBase, int64, error)
	// Update 更新知识库记录
	Update(base *entity.KnowledgeBase) error
	// DeleteByKBID 根据知识库ID删除
	DeleteByKBID(kbID string) error
	// GetStatsByKBIDs 批量获取知识库统计信息
	GetStatsByKBIDs(kbIDs []string) (map[string]*KnowledgeBaseStats, error)
}
