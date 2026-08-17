package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Subscriber 事件订阅者：通过 XREAD BLOCK 从 Run 的 Redis Stream 读取事件
type Subscriber struct {
	client *redis.Client
	block  time.Duration // XREAD 阻塞时长
}

// NewSubscriber 创建事件订阅者
func NewSubscriber(client *redis.Client, block time.Duration) *Subscriber {
	if block <= 0 {
		block = 5 * time.Second
	}
	return &Subscriber{client: client, block: block}
}

// Read 从 afterSeq 之后阻塞读取事件。afterSeq 为 "0-0" 时从头读取。
// 返回的 entries 可能为空（block 超时无新事件），调用方应继续循环。
func (s *Subscriber) Read(ctx context.Context, runID, afterSeq string) ([]StreamEntry, error) {
	if runID == "" {
		return nil, fmt.Errorf("eventbus: run_id is required")
	}
	if afterSeq == "" {
		afterSeq = "0-0"
	}

	streams, err := s.client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{streamKey(runID), afterSeq},
		Count:   100,
		Block:   s.block,
	}).Result()

	if errors.Is(err, redis.Nil) {
		return nil, nil // block 超时，无新事件
	}
	if err != nil {
		return nil, fmt.Errorf("eventbus: xread: %w", err)
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	msgs := streams[0].Messages
	entries := make([]StreamEntry, 0, len(msgs))
	for _, m := range msgs {
		entry, perr := parseEntry(m)
		if perr != nil {
			// 跳过损坏的消息，但保留其 ID 以便续传
			entries = append(entries, StreamEntry{ID: m.ID})
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// LatestID 返回 Run 事件流中最后一条消息的 ID，流不存在时返回 "0-0"
func (s *Subscriber) LatestID(ctx context.Context, runID string) (string, error) {
	msgs, err := s.client.XRevRangeN(ctx, streamKey(runID), "+", "-", 1).Result()
	if err != nil {
		return "", fmt.Errorf("eventbus: xrevrange: %w", err)
	}
	if len(msgs) == 0 {
		return "0-0", nil
	}
	return msgs[0].ID, nil
}

func parseEntry(m redis.XMessage) (StreamEntry, error) {
	raw, ok := m.Values["event"]
	if !ok {
		return StreamEntry{}, fmt.Errorf("eventbus: stream message %s has no event field", m.ID)
	}
	var e Event
	switch v := raw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &e); err != nil {
			return StreamEntry{}, fmt.Errorf("eventbus: unmarshal event: %w", err)
		}
	case []byte:
		if err := json.Unmarshal(v, &e); err != nil {
			return StreamEntry{}, fmt.Errorf("eventbus: unmarshal event: %w", err)
		}
	default:
		return StreamEntry{}, fmt.Errorf("eventbus: unexpected event field type %T", raw)
	}
	return StreamEntry{ID: m.ID, Event: e}, nil
}
