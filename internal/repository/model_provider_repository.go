package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// ModelProviderRepository 模型提供商仓储接口
type ModelProviderRepository interface {
	// FindByID 根据ID查找模型提供商
	FindByID(id uint) (*entity.ModelProvider, error)
	// FindByProviderID 根据ProviderID查找模型提供商
	FindByProviderID(providerID string) (*entity.ModelProvider, error)
	// Create 创建模型提供商
	Create(provider *entity.ModelProvider) error
	// Update 更新模型提供商
	Update(provider *entity.ModelProvider) error
	// Delete 删除模型提供商
	Delete(id uint) error
	// List 分页获取模型提供商列表
	List(offset, limit int, keyword string) ([]*entity.ModelProvider, int64, error)
	// FindEnabledByCapability 根据能力查找启用的模型提供商
	FindEnabledByCapability(capability string) ([]*entity.ModelProvider, error)
}

// modelProviderRepository 模型提供商仓储实现
type modelProviderRepository struct {
	db *gorm.DB
}

// NewModelProviderRepository 创建模型提供商仓储
func NewModelProviderRepository(db *gorm.DB) ModelProviderRepository {
	return &modelProviderRepository{db: db}
}

// FindByID 根据ID查找模型提供商
func (r *modelProviderRepository) FindByID(id uint) (*entity.ModelProvider, error) {
	var provider entity.ModelProvider
	err := r.db.First(&provider, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// FindByProviderID 根据ProviderID查找模型提供商
func (r *modelProviderRepository) FindByProviderID(providerID string) (*entity.ModelProvider, error) {
	var provider entity.ModelProvider
	err := r.db.Where("provider_id = ?", providerID).First(&provider).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// Create 创建模型提供商
func (r *modelProviderRepository) Create(provider *entity.ModelProvider) error {
	return r.db.Create(provider).Error
}

// Update 更新模型提供商
func (r *modelProviderRepository) Update(provider *entity.ModelProvider) error {
	return r.db.Save(provider).Error
}

// Delete 删除模型提供商
func (r *modelProviderRepository) Delete(id uint) error {
	return r.db.Delete(&entity.ModelProvider{}, id).Error
}

// List 分页获取模型提供商列表
func (r *modelProviderRepository) List(offset, limit int, keyword string) ([]*entity.ModelProvider, int64, error) {
	var providers []*entity.ModelProvider
	var total int64

	query := r.db.Model(&entity.ModelProvider{})
	if keyword != "" {
		query = query.Where("display_name LIKE ? OR provider_id LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&providers).Error
	if err != nil {
		return nil, 0, err
	}

	return providers, total, nil
}

// FindEnabledByCapability 根据能力查找启用的模型提供商
func (r *modelProviderRepository) FindEnabledByCapability(capability string) ([]*entity.ModelProvider, error) {
	var providers []*entity.ModelProvider
	err := r.db.Where("is_enabled = ? AND JSON_CONTAINS(capabilities, ?)",
		true, `"`+capability+`"`).Find(&providers).Error
	return providers, err
}