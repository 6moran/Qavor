package request

import (
	"Qavor/pkg/response"
)

// UpdateUserRequest 更新用户信息请求
type UpdateUserRequest struct {
	Avatar string `json:"avatar" binding:"omitempty,url,max=255"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=50"`
}

// UserListRequest 用户列表请求
type UserListRequest struct {
	response.PageRequest
}
