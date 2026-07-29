package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"Qavor/pkg/database"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// TokenBlacklistKey 返回不包含原始 JWT 的 Redis 黑名单键。
func TokenBlacklistKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return TokenBlacklistPrefix + hex.EncodeToString(digest[:])
}

const (
	// ResetCodePrefix 重置码前缀
	ResetCodePrefix = "reset_code:"
	// TokenBlacklistPrefix Token 黑名单前缀
	TokenBlacklistPrefix = "token_blacklist:"
	// ResetCodeLength 验证码长度
	ResetCodeLength = 6
	// ResetCodeExpire 验证码有效期（10分钟）
	ResetCodeExpire = 10 * time.Minute
)

// GenerateResetCode 生成数字验证码
//
// 返回:
//   - string: 验证码字符串
func GenerateResetCode() string {
	Min := 1
	Max := 9
	for i := 1; i < ResetCodeLength; i++ {
		Min *= 10
		Max = Max*10 + 9
	}
	code := rand.Intn(Max-Min+1) + Min
	return strconv.Itoa(code)
}

// SaveResetCode 保存验证码到 Redis
// 参数:
//   - email: 用户邮箱
//   - code: 验证码
//
// 返回:
//   - error: 保存失败时返回错误
func SaveResetCode(email, code string) error {
	ctx := context.Background()
	key := ResetCodePrefix + email

	// 保存验证码，设置过期时间
	err := database.GetRedis().Set(ctx, key, code, ResetCodeExpire).Err()
	if err != nil {
		logger.Error("保存验证码失败", zap.Error(err))
		return fmt.Errorf("保存验证码失败: %w", err)
	}

	logger.Info("验证码已保存", zap.String("email", email))
	return nil
}

// GetResetCode 获取验证码
// 参数:
//   - email: 用户邮箱
//
// 返回:
//   - string: 验证码（如果不存在或已过期返回空字符串）
//   - error: 获取失败时返回错误
func GetResetCode(email string) (string, error) {
	ctx := context.Background()
	key := ResetCodePrefix + email

	code, err := database.GetRedis().Get(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return "", nil // 验证码不存在或已过期
		}
		logger.Error("获取验证码失败", zap.Error(err))
		return "", fmt.Errorf("获取验证码失败: %w", err)
	}

	return code, nil
}

// DeleteResetCode 删除验证码
// 参数:
//   - email: 用户邮箱
//
// 返回:
//   - error: 删除失败时返回错误
func DeleteResetCode(email string) error {
	ctx := context.Background()
	key := ResetCodePrefix + email

	err := database.GetRedis().Del(ctx, key).Err()
	if err != nil {
		logger.Error("删除验证码失败", zap.Error(err))
		return fmt.Errorf("删除验证码失败: %w", err)
	}

	return nil
}

// VerifyResetCode 验证验证码
// 参数:
//   - email: 用户邮箱
//   - code: 用户输入的验证码
//
// 返回:
//   - bool: 验证码是否有效
//   - error: 验证失败时返回错误
func VerifyResetCode(email, code string) (bool, error) {
	storedCode, err := GetResetCode(email)
	if err != nil {
		return false, err
	}

	if storedCode == "" {
		return false, nil // 验证码不存在或已过期
	}

	return storedCode == code, nil
}

// AddTokenToBlacklist 将 Token 加入黑名单
// 参数:
//   - token: 需要加入黑名单的 Token
//   - expireTime: Token 过期时间（用于设置黑名单过期时间）
//
// 返回:
//   - error: 操作失败时返回错误
func AddTokenToBlacklist(token string, expireTime time.Duration) error {
	if !database.RedisAvailable() {
		return fmt.Errorf("Redis 未初始化")
	}
	ctx := context.Background()
	key := TokenBlacklistKey(token)

	// 保存 Token 到黑名单，设置过期时间
	err := database.GetRedis().Set(ctx, key, "1", expireTime).Err()
	if err != nil {
		logger.Error("将 Token 加入黑名单失败", zap.Error(err))
		return fmt.Errorf("将 Token 加入黑名单失败: %w", err)
	}

	logger.Info("Token 已加入黑名单")
	return nil
}

// IsTokenInBlacklist 检查 Token 是否在黑名单中
// 参数:
//   - token: 需要检查的 Token
//
// 返回:
//   - bool: Token 是否在黑名单中
//   - error: 检查失败时返回错误
func IsTokenInBlacklist(token string) (bool, error) {
	if !database.RedisAvailable() {
		return false, fmt.Errorf("Redis 未初始化")
	}
	ctx := context.Background()
	key := TokenBlacklistKey(token)

	exists, err := database.GetRedis().Exists(ctx, key).Result()
	if err != nil {
		logger.Error("检查 Token 黑名单失败", zap.Error(err))
		return false, fmt.Errorf("检查 Token 黑名单失败: %w", err)
	}

	return exists > 0, nil
}
