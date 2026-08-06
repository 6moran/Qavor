package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Publisher 事件发布者：将事件通过 XADD 写入 Run 的 Redis Stream
type Publisher struct {
	client *redis.Client
	maxLen int64 // Stream 近似最大长度
}

// NewPublisher 创建事件发布者
func NewPublisher(client *redis.Client, maxLen int64) *Publisher {
	if maxLen <= 0 {
		maxLen = 10000
	}
	return &Publisher{client: client, maxLen: maxLen}
}

// Publish 将一个事件写入指定 Run 的事件流，返回 Redis Stream 的 entry ID
func (p *Publisher) Publish(ctx context.Context, e Event) (string, error) {
	if e.RunID == "" {
		return "", fmt.Errorf("eventbus: run_id is required")
	}
	data, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("eventbus: marshal event: %w", err)
	}
	id, err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(e.RunID),
		MaxLen: p.maxLen,
		Approx: true,
		Values: map[string]any{"event": string(data)},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("eventbus: xadd: %w", err)
	}
	return id, nil
}

// PublishPayload 便捷方法：构造事件并发布
func (p *Publisher) PublishPayload(ctx context.Context, eventType, runID, threadID, requestID string, payload any) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("eventbus: marshal payload: %w", err)
	}
	return p.Publish(ctx, Event{
		EventType: eventType,
		RunID:     runID,
		ThreadID:  threadID,
		RequestID: requestID,
		Payload:   raw,
	})
}

// MaxLen 返回配置的最大长度
func (p *Publisher) MaxLen() int64 { return p.maxLen }
