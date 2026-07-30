package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// ModelRepository 模型仓储接口
type ModelRepository interface {
	// FindByID 根据ID查找模型
	FindByID(id uint) (*entity.Model, error)
	// Create 创建模型
	Create(model *entity.Model) error
	// Update 更新模型
	Update(model *entity.Model) error
	// Delete 删除模型
	Delete(id uint) error
	// List 分页获取模型列表
	List(offset, limit int, keyword string, modelType string) ([]*entity.Model, int64, error)
}

// modelRepository 模型仓储实现
type modelRepository struct {
	db *gorm.DB
}

// NewModelRepository 创建模型仓储
func NewModelRepository(db *gorm.DB) ModelRepository {
	return &modelRepository{db: db}
}

// FindByID 根据ID查找模型
func (r *modelRepository) FindByID(id uint) (*entity.Model, error) {
	var model entity.Model
	err := r.db.First(&model, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

// Create 创建模型
func (r *modelRepository) Create(model *entity.Model) error {
	return r.db.Create(model).Error
}

// Update 更新模型
func (r *modelRepository) Update(model *entity.Model) error {
	return r.db.Save(model).Error
}

// Delete 删除模型
func (r *modelRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Model{}, id).Error
}

// List 分页获取模型列表
func (r *modelRepository) List(offset, limit int, keyword string, modelType string) ([]*entity.Model, int64, error) {
	var models []*entity.Model
	var total int64

	query := r.db.Model(&entity.Model{})
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if modelType != "" {
		query = query.Where("model_type = ?", modelType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&models).Error
	if err != nil {
		return nil, 0, err
	}

	return models, total, nil
}
