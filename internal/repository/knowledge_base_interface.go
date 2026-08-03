package repository

import "Qavor/internal/model/entity"

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
}
