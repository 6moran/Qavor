package service

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
)

// AuthService 认证服务接口
type AuthService interface {
	// Login 单实例管理员登录。
	Login(req *request.LoginRequest) (*dto.LoginResponse, error)
	// Logout 使当前 JWT 立即失效；黑名单存储不可用时降级为自然过期。
	Logout(token string) error
}
