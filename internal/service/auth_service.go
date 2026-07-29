package service

import (
	"crypto/subtle"
	"time"

	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/pkg/cache"
	"Qavor/pkg/config"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/jwt"
	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

var (
	parseToken                = jwt.ParseToken
	blacklistToken            = cache.AddTokenToBlacklist
	warnBlacklistWriteFailure = func(err error) {
		logger.Warn("Token 黑名单写入失败，降级为自然过期", zap.Error(err))
	}
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

// Logout 使当前 JWT 进入黑名单直到其自然过期。
func (s *authService) Logout(token string) error {
	claims, err := parseToken(token)
	if err != nil {
		return err
	}

	ttl := jwt.RemainingTTL(claims, time.Now())
	if ttl == 0 {
		return nil
	}
	if err := blacklistToken(token, ttl); err != nil {
		warnBlacklistWriteFailure(err)
	}
	return nil
}
