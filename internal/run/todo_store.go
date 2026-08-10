package run

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"Qavor/internal/eventbus"

	"github.com/redis/go-redis/v9"
)

// TodoStore 基于 Redis 的 TODO 列表持久化（按会话隔离）
type TodoStore struct {
	redis *redis.Client
	ttl   time.Duration
}

// NewTodoStore 创建 TodoStore
func NewTodoStore(rdb *redis.Client, ttl time.Duration) *TodoStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &TodoStore{redis: rdb, ttl: ttl}
}

// todoKey Redis 键
func todoKey(conversationID uint) string {
	return fmt.Sprintf("agent:todos:%d", conversationID)
}

// SaveTodos 保存最新 TODO 列表（覆盖写）
func (s *TodoStore) SaveTodos(ctx context.Context, conversationID uint, todos []eventbus.TodoItem) error {
	if s == nil || s.redis == nil || conversationID == 0 {
		return nil
	}
	data, err := json.Marshal(todos)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, todoKey(conversationID), data, s.ttl).Err()
}

// GetTodos 读取 TODO 列表
func (s *TodoStore) GetTodos(ctx context.Context, conversationID uint) ([]eventbus.TodoItem, error) {
	if s == nil || s.redis == nil || conversationID == 0 {
		return nil, nil
	}
	data, err := s.redis.Get(ctx, todoKey(conversationID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var todos []eventbus.TodoItem
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}
