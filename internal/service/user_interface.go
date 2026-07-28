package service

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/pkg/response"

	"Qavor/internal/model/entity"
)

// UserService 用户服务接口
type UserService interface {
	// Register 用户注册
	Register(req *request.RegisterRequest) error
	// GetUserByUID 根据 UID 获取用户
	GetUserByUID(uid string) (*entity.User, error)
	// UpdateUser 更新用户信息
	UpdateUser(uid string, req *request.UpdateUserRequest) error
	// ChangePassword 修改密码
	ChangePassword(uid string, req *request.ChangePasswordRequest) error
	// GetUserResponse 获取用户响应
	GetUserResponse(user *entity.User) *dto.UserResponse
	// ListUsers 分页获取用户列表
	ListUsers(req *request.UserListRequest) (*response.PageResponse, error)
}
