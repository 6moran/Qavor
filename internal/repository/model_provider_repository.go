package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// ModelProviderRepository 模型供应商仓储接口
type ModelProviderRepository interface {
	// FindByID 根据ID查找模型供应商
	FindByID(id uint) (*entity.Model, error)
	// Create 创建模型供应商
	Create(model *entity.Model) error
	// Update 更新模型供应商
	Update(model *entity.Model) error
	// Delete 删除模型供应商
	Delete(id uint) error
	// List 分页获取模型供应商列表
	List(offset, limit int, keyword string) ([]*entity.Model, int64, error)
}

// modelProviderRepository 模型供应商仓储实现
type modelProviderRepository struct {
	db *gorm.DB
}

// NewModelProviderRepository 创建模型供应商仓储
func NewModelProviderRepository(db *gorm.DB) ModelProviderRepository {
	return &modelProviderRepository{db: db}
}

// FindByID 根据ID查找模型供应商
func (r *modelProviderRepository) FindByID(id uint) (*entity.Model, error) {
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

// Create 创建模型供应商
func (r *modelProviderRepository) Create(model *entity.Model) error {
	return r.db.Create(model).Error
}

// Update 更新模型供应商
func (r *modelProviderRepository) Update(model *entity.Model) error {
	return r.db.Save(model).Error
}

// Delete 删除模型供应商
func (r *modelProviderRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Model{}, id).Error
}

// List 分页获取模型供应商列表
func (r *modelProviderRepository) List(offset, limit int, keyword string) ([]*entity.Model, int64, error) {
	var models []*entity.Model
	var total int64

	query := r.db.Model(&entity.Model{})
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
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
