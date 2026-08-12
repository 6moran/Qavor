package repository

import (
	"context"
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type systemSettingRepository struct {
	db *gorm.DB
}

// NewSystemSettingRepository 创建系统设置仓储。
func NewSystemSettingRepository(db *gorm.DB) SystemSettingRepository {
	return &systemSettingRepository{db: db}
}

// Get 读取指定键；键不存在时返回 found=false。
func (r *systemSettingRepository) Get(ctx context.Context, key string) (string, bool, error) {
	var setting entity.SystemSetting
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return setting.Value, true, nil
}

// Upsert 原子创建或更新指定键。
func (r *systemSettingRepository) Upsert(ctx context.Context, key, value string) error {
	setting := &entity.SystemSetting{Key: key, Value: value}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(setting).Error
}
