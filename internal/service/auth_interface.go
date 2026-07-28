package service

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
)

// AuthService 认证服务接口
type AuthService interface {
	// Login 用户登录
	Login(req *request.LoginRequest) (*dto.LoginResponse, error)
	// Logout 用户登出
	Logout(accessToken, refreshToken string) error
	// RefreshToken 刷新访问令牌
	RefreshToken(refreshToken string) (*dto.TokenRefreshResponse, error)
	// SendResetCode 发送重置验证码
	SendResetCode(req *request.SendResetCodeRequest) (*dto.ResetCodeResponse, error)
	// ResetPassword 重置密码（包含验证码验证）
	ResetPassword(req *request.ResetPasswordRequest) error
}
