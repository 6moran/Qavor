package repository

import (
	"Qavor/internal/model/entity"
)

// UserRepository 用户仓储接口
type UserRepository interface {
	// FindByID 根据 ID 查找用户
	FindByID(id uint) (*entity.User, error)
	// FindByUsername 根据用户名查找用户
	FindByUsername(username string) (*entity.User, error)
	// FindByEmail 根据邮箱查找用户
	FindByEmail(email string) (*entity.User, error)
	// Create 创建用户
	Create(user *entity.User) error
	// Update 更新用户
	Update(user *entity.User) error
	// Delete 删除用户
	Delete(id uint) error
	// List 分页获取用户列表
	List(offset, limit int) ([]*entity.User, int64, error)
	// ExistsByUsername 检查用户名是否存在
	ExistsByUsername(username string) (bool, error)
	// ExistsByEmail 检查邮箱是否存在
	ExistsByEmail(email string) (bool, error)
}
