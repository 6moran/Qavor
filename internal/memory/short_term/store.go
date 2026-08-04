package shortterm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisStore Redis 存储
type RedisStore struct {
	client *redis.Client
	logger *zap.Logger
	ttl    time.Duration // 过期时间
}

// NewRedisStore 创建 Redis 存储
func NewRedisStore(client *redis.Client, logger *zap.Logger, ttl time.Duration) *RedisStore {
	if ttl == 0 {
		ttl = 24 * time.Hour // 默认24小时
	}
	return &RedisStore{
		client: client,
		logger: logger,
		ttl:    ttl,
	}
}

// key 生成 Redis key
func (s *RedisStore) key(conversationID uint) string {
	return fmt.Sprintf("memory:short_term:%d", conversationID)
}

// Save 保存短期记忆
func (s *RedisStore) Save(ctx context.Context, memory *SessionMemory) error {
	key := s.key(memory.ConversationID)
	data, err := json.Marshal(memory)
	if err != nil {
		s.logger.Error("序列化短期记忆失败", zap.Error(err))
		return err
	}

	// 保存并刷新 TTL
	if err := s.client.Set(ctx, key, data, s.ttl).Err(); err != nil {
		s.logger.Error("保存短期记忆失败", zap.Error(err))
		return err
	}

	return nil
}

// Load 加载短期记忆
func (s *RedisStore) Load(ctx context.Context, conversationID uint) (*SessionMemory, error) {
	key := s.key(conversationID)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // 不存在
		}
		s.logger.Error("加载短期记忆失败", zap.Error(err))
		return nil, err
	}

	var memory SessionMemory
	if err := json.Unmarshal(data, &memory); err != nil {
		s.logger.Error("反序列化短期记忆失败", zap.Error(err))
		return nil, err
	}

	return &memory, nil
}

// Delete 删除短期记忆
func (s *RedisStore) Delete(ctx context.Context, conversationID uint) error {
	key := s.key(conversationID)
	return s.client.Del(ctx, key).Err()
}

// RefreshTTL 刷新 TTL
func (s *RedisStore) RefreshTTL(ctx context.Context, conversationID uint) error {
	key := s.key(conversationID)
	return s.client.Expire(ctx, key, s.ttl).Err()
}

// Exists 检查是否存在
func (s *RedisStore) Exists(ctx context.Context, conversationID uint) (bool, error) {
	key := s.key(conversationID)
	count, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
