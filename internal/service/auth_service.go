package service

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/repository"
	"Qavor/pkg/cache"
	"Qavor/pkg/config"
	"Qavor/pkg/email"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/jwt"
	"Qavor/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// authService 认证服务实现
type authService struct {
	userRepo    repository.UserRepository
	userService UserService
	emailClient *email.SMTPClient
}

// NewAuthService 创建认证服务
// 参数:
//   - userRepo: 用户仓储
//   - userService: 用户服务
//   - emailClient: 邮件客户端
//
// 返回:
//   - AuthService: 认证服务接口
func NewAuthService(
	userRepo repository.UserRepository,
	userService UserService,
	emailClient *email.SMTPClient,
) AuthService {
	return &authService{
		userRepo:    userRepo,
		userService: userService,
		emailClient: emailClient,
	}
}

// Login 用户登录
// 参数:
//   - req: 登录请求，包含邮箱和密码
//
// 返回:
//   - *dto.LoginResponse: 登录响应，包含 Token 和用户信息
//   - error: 登录失败时返回错误
func (s *authService) Login(req *request.LoginRequest) (*dto.LoginResponse, error) {
	// 1. 检查是否首次运行（系统中是否有用户）
	_, total, err := s.userRepo.List(0, 1)
	if err != nil {
		return nil, err
	}
	isFirstRun := total == 0

	// 首次运行，返回标志但不生成 Token
	if isFirstRun {
		return &dto.LoginResponse{
			IsFirstRun: true,
		}, nil
	}

	// 2. 查找用户
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 3. 检查用户状态
	if user.Status != 1 {
		return nil, bizerrors.ErrUserDisabled
	}

	// 4. 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 5. 生成 Token 对
	tokenPair, err := jwt.GenerateTokenPair(user.UID)
	if err != nil {
		return nil, err
	}

	// 6. 构建响应
	userResp := s.userService.GetUserResponse(user)
	return &dto.LoginResponse{
		AccessToken:      tokenPair.AccessToken,
		RefreshToken:     tokenPair.RefreshToken,
		AccessExpiresIn:  tokenPair.AccessExpiresIn,
		RefreshExpiresIn: tokenPair.RefreshExpiresIn,
		ExpiresIn:        tokenPair.AccessExpiresIn, // 兼容字段
		IsFirstRun:       false,
		User:             *userResp,
	}, nil
}

// Logout 用户登出
// 参数:
//   - accessToken: 访问令牌
//   - refreshToken: 刷新令牌
//
// 返回:
//   - error: 登出失败时返回错误
func (s *authService) Logout(accessToken, refreshToken string) error {
	// 解析访问令牌获取过期时间
	if accessToken != "" {
		claims, err := jwt.ParseToken(accessToken)
		if err == nil && claims.ExpiresAt != nil {
			// 计算 Token 剩余有效时间
			expireTime := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
			// 将访问令牌加入黑名单
			if err := cache.AddTokenToBlacklist(accessToken, expireTime); err != nil {
				logger.Warn("将访问令牌加入黑名单失败", zap.Error(err))
			}
		}
	}

	// 解析刷新令牌获取过期时间
	if refreshToken != "" {
		claims, err := jwt.ParseToken(refreshToken)
		if err == nil && claims.ExpiresAt != nil {
			// 计算 Token 剩余有效时间
			expireTime := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
			// 将刷新令牌加入黑名单
			if err := cache.AddTokenToBlacklist(refreshToken, expireTime); err != nil {
				logger.Warn("将刷新令牌加入黑名单失败", zap.Error(err))
			}
		}
	}

	logger.Info("用户登出成功")
	return nil
}

// RefreshToken 刷新访问令牌
// 参数:
//   - refreshToken: 刷新令牌
//
// 返回:
//   - *dto.TokenRefreshResponse: Token 刷新响应，包含新的访问令牌和刷新令牌
//   - error: 刷新失败时返回错误
func (s *authService) RefreshToken(refreshToken string) (*dto.TokenRefreshResponse, error) {
	// 检查刷新令牌是否在黑名单中
	inBlacklist, err := cache.IsTokenInBlacklist(refreshToken)
	if err != nil {
		return nil, err
	}
	if inBlacklist {
		return nil, bizerrors.ErrInvalidToken
	}

	// 解析刷新令牌
	claims, err := jwt.ParseToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// 验证是否为刷新令牌
	if claims.Subject != "refresh" {
		return nil, bizerrors.ErrInvalidToken
	}

	// 生成新的 Token 对
	newTokenPair, err := jwt.GenerateTokenPair(claims.UID)
	if err != nil {
		return nil, err
	}

	// 将旧的刷新令牌加入黑名单
	cfg := config.Get()
	expireTime := cfg.JWT.RefreshExpire * 3600 * 1000000000 // 转换为纳秒
	if err := cache.AddTokenToBlacklist(refreshToken, expireTime); err != nil {
		logger.Warn("将旧刷新令牌加入黑名单失败", zap.Error(err))
	}

	return &dto.TokenRefreshResponse{
		AccessToken:      newTokenPair.AccessToken,
		RefreshToken:     newTokenPair.RefreshToken,
		AccessExpiresIn:  newTokenPair.AccessExpiresIn,
		RefreshExpiresIn: newTokenPair.RefreshExpiresIn,
	}, nil
}

// SendResetCode 发送重置验证码
// 参数:
//   - req: 发送验证码请求，包含邮箱
//
// 返回:
//   - *dto.ResetCodeResponse: 响应，包含验证码有效期
//   - error: 发送失败时返回错误
func (s *authService) SendResetCode(req *request.SendResetCodeRequest) (*dto.ResetCodeResponse, error) {
	// 1. 检查邮箱是否存在
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrEmailNotExists
	}

	// 2. 生成 6 位数字验证码
	code := cache.GenerateResetCode()

	// 3. 保存验证码到 Redis
	if err := cache.SaveResetCode(req.Email, code); err != nil {
		logger.Error("保存验证码失败", zap.Error(err))
		return nil, bizerrors.New(bizerrors.CodeInternalError, "发送验证码失败")
	}

	// 4. 构建邮件内容
	subject, body := email.BuildResetCodeEmail(code)

	// 5. 发送邮件
	if err := s.emailClient.Send([]string{req.Email}, subject, body); err != nil {
		logger.Error("发送验证码邮件失败", zap.Error(err))
		return nil, bizerrors.New(bizerrors.CodeInternalError, "发送验证码失败")
	}

	logger.Info("验证码已发送", zap.String("email", req.Email))

	return &dto.ResetCodeResponse{
		ExpiresIn: 600, // 10 分钟
	}, nil
}

// ResetPassword 重置密码（包含验证码验证）
func (s *authService) ResetPassword(req *request.ResetPasswordRequest) error {
	// 1. 验证验证码
	valid, err := cache.VerifyResetCode(req.Email, req.Code)
	if err != nil {
		return err
	}
	if !valid {
		return bizerrors.ErrInvalidResetCode
	}

	// 2. 查找用户
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return err
	}
	if user == nil {
		return bizerrors.ErrUserNotFound
	}

	// 3. 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 4. 更新密码
	user.Password = string(hashedPassword)
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// 5. 删除已使用的验证码
	if err := cache.DeleteResetCode(req.Email); err != nil {
		logger.Warn("删除验证码失败", zap.Error(err))
	}

	// 6. 发送密码重置成功邮件
	subject, body := email.BuildResetSuccessEmail()
	if err := s.emailClient.Send([]string{req.Email}, subject, body); err != nil {
		logger.Warn("发送密码重置成功邮件失败", zap.Error(err))
	}

	logger.Info("密码重置成功", zap.String("email", req.Email))
	return nil
}
