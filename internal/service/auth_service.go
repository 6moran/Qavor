package service

import (
	"crypto/subtle"

	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/pkg/config"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/jwt"
)

// authService 认证服务实现
type authService struct {
	authConfig config.AuthConfig
}

// NewAuthService 创建认证服务
func NewAuthService(authConfig config.AuthConfig) AuthService {
	return &authService{
		authConfig: authConfig,
	}
}

// Login 用户登录
func (s *authService) Login(req *request.LoginRequest) (*dto.LoginResponse, error) {
	usernameMatches := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.authConfig.AdminUsername)) == 1
	passwordMatches := subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.authConfig.AdminPassword)) == 1
	if !usernameMatches || !passwordMatches {
		return nil, bizerrors.ErrInvalidCredentials
	}

	token, err := jwt.GenerateToken()
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{
		Token: token,
	}, nil
}

// RefreshToken 刷新 Token
func (s *authService) RefreshToken(token string) (string, error) {
	newToken, err := jwt.RefreshToken(token)
	if err != nil {
		return "", err
	}
	return newToken, nil
}
