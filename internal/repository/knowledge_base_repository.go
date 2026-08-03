package repository

import (
	"errors"
	"strings"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// knowledgeBaseRepository 知识库数据访问实现
type knowledgeBaseRepository struct {
	db *gorm.DB
}

// NewKnowledgeBaseRepository 创建知识库数据访问实例
func NewKnowledgeBaseRepository(db *gorm.DB) KnowledgeBaseRepository {
	return &knowledgeBaseRepository{db: db}
}

// Create 创建知识库记录
func (r *knowledgeBaseRepository) Create(base *entity.KnowledgeBase) error {
	return r.db.Create(base).Error
}

// FindByKBID 根据知识库ID查询
func (r *knowledgeBaseRepository) FindByKBID(kbID string) (*entity.KnowledgeBase, error) {
	var base entity.KnowledgeBase
	// 查询单条记录，记录不存在时返回 nil 而非错误
	err := r.db.Where("kb_id = ?", kbID).First(&base).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &base, nil
}

// List 分页查询知识库列表
func (r *knowledgeBaseRepository) List(offset, limit int, keyword string) ([]*entity.KnowledgeBase, int64, error) {
	// 构建查询条件
	query := r.db.Model(&entity.KnowledgeBase{})
	// 关键词搜索（模糊匹配名称和描述）
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	// 查询总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 分页查询列表，按创建时间倒序
	var bases []*entity.KnowledgeBase
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&bases).Error; err != nil {
		return nil, 0, err
	}
	return bases, total, nil
}

// Update 更新知识库记录
func (r *knowledgeBaseRepository) Update(base *entity.KnowledgeBase) error {
	return r.db.Save(base).Error
}

// DeleteByKBID 根据知识库ID删除
func (r *knowledgeBaseRepository) DeleteByKBID(kbID string) error {
	return r.db.Where("kb_id = ?", kbID).Delete(&entity.KnowledgeBase{}).Error
}
