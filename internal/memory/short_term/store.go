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
// TTL 定位：兜底清理机制，生命周期由会话关闭/归档控制
type RedisStore struct {
	client *redis.Client
	logger *zap.Logger
	ttl    time.Duration // 兜底过期时间
}

// NewRedisStore 创建 Redis 存储
// 默认 TTL 168h（7天），作为兜底清理；正常生命周期由显式关闭/归档控制
func NewRedisStore(client *redis.Client, logger *zap.Logger, ttl time.Duration) *RedisStore {
	if ttl == 0 {
		ttl = 168 * time.Hour // 7天兜底
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

// Save 保存短期上下文（每次保存刷新 TTL）
func (s *RedisStore) Save(ctx context.Context, memory *SessionContext) error {
	key := s.key(memory.ConversationID)
	data, err := json.Marshal(memory)
	if err != nil {
		s.logger.Error("序列化短期上下文失败", zap.Error(err))
		return err
	}

	if err := s.client.Set(ctx, key, data, s.ttl).Err(); err != nil {
		s.logger.Error("保存短期上下文失败", zap.Error(err))
		return err
	}

	return nil
}

// Load 加载短期上下文
func (s *RedisStore) Load(ctx context.Context, conversationID uint) (*SessionContext, error) {
	key := s.key(conversationID)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		s.logger.Error("加载短期上下文失败", zap.Error(err))
		return nil, err
	}

	var memory SessionContext
	if err := json.Unmarshal(data, &memory); err != nil {
		s.logger.Error("反序列化短期上下文失败", zap.Error(err))
		return nil, err
	}

	return &memory, nil
}

// Delete 删除短期上下文
func (s *RedisStore) Delete(ctx context.Context, conversationID uint) error {
	key := s.key(conversationID)
	return s.client.Del(ctx, key).Err()
}
