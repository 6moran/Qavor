package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"Qavor/pkg/database"
	"Qavor/pkg/logger"
	"go.uber.org/zap"
)

const TokenBlacklistPrefix = "token_blacklist:"

// TokenBlacklistKey 返回不包含原始 JWT 的 Redis 黑名单键。
func TokenBlacklistKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return TokenBlacklistPrefix + hex.EncodeToString(digest[:])
}

// AddTokenToBlacklist 将 Token 加入黑名单。
func AddTokenToBlacklist(token string, expireTime time.Duration) error {
	if !database.RedisAvailable() {
		return fmt.Errorf("Redis 未初始化")
	}
	ctx := context.Background()
	key := TokenBlacklistKey(token)

	if err := database.GetRedis().Set(ctx, key, "1", expireTime).Err(); err != nil {
		logger.Error("将 Token 加入黑名单失败", zap.Error(err))
		return fmt.Errorf("将 Token 加入黑名单失败: %w", err)
	}

	logger.Info("Token 已加入黑名单")
	return nil
}

// IsTokenInBlacklist 检查 Token 是否在黑名单中。
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
