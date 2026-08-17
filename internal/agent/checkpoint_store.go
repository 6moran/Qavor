package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/redis/go-redis/v9"
)

// checkpointKeyPrefix Redis checkpoint key 前缀。
const checkpointKeyPrefix = "qavor:checkpoint:"

// redisCmdable 抽象 go-redis 的命令接口，使 store 逻辑可在无 Redis 环境测试。
type redisCmdable interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// RedisCheckPointStore 基于 Redis 的 eino checkpoint 存储。
// 存储 agent 执行中断时的整棵执行图快照（eino 框架内部 gob 序列化后的字节）。
// 跨进程持久化，使审批恢复可跨请求/跨重启（Redis 存活时）。
type RedisCheckPointStore struct {
	client redisCmdable
	ttl    time.Duration // checkpoint 过期时间
}

// NewRedisCheckPointStore 创建 Redis checkpoint 存储。
// client 为 go-redis 客户端（*redis.Client）；ttl 为过期时间（0=不过期）。
func NewRedisCheckPointStore(client redisCmdable, ttl time.Duration) *RedisCheckPointStore {
	return &RedisCheckPointStore{client: client, ttl: ttl}
}

// key 生成 checkpoint 的 Redis key。
func (s *RedisCheckPointStore) key(checkPointID string) string {
	return checkpointKeyPrefix + checkPointID
}

// Get 读取 checkpoint。key 不存在时返回 ok=false（Redis 返回 redis.Nil）。
func (s *RedisCheckPointStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	val, err := s.client.Get(ctx, s.key(checkPointID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get checkpoint %s: %w", checkPointID, err)
	}
	return val, true, nil
}

// Set 写入 checkpoint。ttl 非零时设置过期。
func (s *RedisCheckPointStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	if err := s.client.Set(ctx, s.key(checkPointID), checkPoint, s.ttl).Err(); err != nil {
		return fmt.Errorf("redis set checkpoint %s: %w", checkPointID, err)
	}
	return nil
}

// Delete 删除 checkpoint（实现 core.CheckPointDeleter，审批结束后清理）。
func (s *RedisCheckPointStore) Delete(ctx context.Context, checkPointID string) error {
	if err := s.client.Del(ctx, s.key(checkPointID)).Err(); err != nil {
		return fmt.Errorf("redis del checkpoint %s: %w", checkPointID, err)
	}
	return nil
}

var _ adk.CheckPointStore = (*RedisCheckPointStore)(nil)
