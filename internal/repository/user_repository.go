package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// userRepository 用户仓储实现
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// FindByID 根据 ID 查找用户
func (r *userRepository) FindByID(id uint) (*entity.User, error) {
	var user entity.User
	err := r.db.First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByUID 根据 UID 查找用户
func (r *userRepository) FindByUID(uid string) (*entity.User, error) {
	var user entity.User
	err := r.db.Where("uid = ?", uid).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByEmail 根据邮箱查找用户
func (r *userRepository) FindByEmail(email string) (*entity.User, error) {
	var user entity.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Create 创建用户
func (r *userRepository) Create(user *entity.User) error {
	return r.db.Create(user).Error
}

// Update 更新用户
func (r *userRepository) Update(user *entity.User) error {
	return r.db.Save(user).Error
}

// Delete 删除用户
func (r *userRepository) Delete(id uint) error {
	return r.db.Delete(&entity.User{}, id).Error
}

// List 分页获取用户列表
func (r *userRepository) List(offset, limit int) ([]*entity.User, int64, error) {
	var users []*entity.User
	var total int64

	// 统计总数
	if err := r.db.Model(&entity.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err := r.db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// ExistsByEmail 检查邮箱是否存在
func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}
