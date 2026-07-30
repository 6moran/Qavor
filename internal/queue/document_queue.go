package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Message identifies one document-processing job delivered through Redis.
type Message struct {
	ID        string
	JobID     string
	KBID      string
	FileID    string
	CreatedAt time.Time
	Schema    int
	// InvalidReason is set when Redis supplied a malformed payload. ID remains
	// available so the worker can acknowledge the poison message.
	InvalidReason string
}

// DocumentQueue is the message transport used by document-processing services.
type DocumentQueue interface {
	EnsureGroup(ctx context.Context) error
	Publish(ctx context.Context, message Message) error
	Consume(ctx context.Context, consumer string, block time.Duration) (*Message, error)
	Ack(ctx context.Context, messageID string) error
	ClaimStale(ctx context.Context, consumer string, minIdle time.Duration, count int64) ([]Message, error)
}

type streamClient interface {
	XGroupCreateMkStream(context.Context, string, string, string) *redis.StatusCmd
	XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd
	XReadGroup(context.Context, *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XAck(context.Context, string, string, ...string) *redis.IntCmd
	XPendingExt(context.Context, *redis.XPendingExtArgs) *redis.XPendingExtCmd
	XClaim(context.Context, *redis.XClaimArgs) *redis.XMessageSliceCmd
}

type redisDocumentQueue struct {
	client streamClient
	stream string
	group  string
	maxLen int64
}

// NewRedisDocumentQueue creates a Redis Streams-backed document queue.
func NewRedisDocumentQueue(client *redis.Client, stream, group string, maxLen int64) (DocumentQueue, error) {
	if client == nil {
		return nil, errors.New("redis client is unavailable")
	}
	if strings.TrimSpace(stream) == "" {
		return nil, errors.New("document queue stream is required")
	}
	if strings.TrimSpace(group) == "" {
		return nil, errors.New("document queue consumer group is required")
	}
	if maxLen <= 0 {
		maxLen = 100000
	}
	return &redisDocumentQueue{client: client, stream: stream, group: group, maxLen: maxLen}, nil
}

func (q *redisDocumentQueue) EnsureGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, q.stream, q.group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("create document queue consumer group: %w", err)
	}
	return nil
}

func (q *redisDocumentQueue) Publish(ctx context.Context, message Message) error {
	if strings.TrimSpace(message.JobID) == "" {
		return errors.New("document queue message job_id is required")
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}
	if message.Schema <= 0 {
		message.Schema = 1
	}
	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		MaxLen: q.maxLen,
		Approx: true,
		Values: map[string]any{
			"job_id":     message.JobID,
			"kb_id":      message.KBID,
			"file_id":    message.FileID,
			"created_at": message.CreatedAt.UTC().Format(time.RFC3339Nano),
			"schema":     message.Schema,
		},
	}).Err()
}

func (q *redisDocumentQueue) Consume(ctx context.Context, consumer string, block time.Duration) (*Message, error) {
	if strings.TrimSpace(consumer) == "" {
		return nil, errors.New("document queue consumer is required")
	}
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: consumer,
		Streams:  []string{q.stream, ">"},
		Count:    1,
		Block:    block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consume document queue message: %w", err)
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}
	message, err := parseMessage(streams[0].Messages[0])
	if err != nil {
		return &Message{ID: streams[0].Messages[0].ID, InvalidReason: err.Error()}, nil
	}
	return &message, nil
}

func (q *redisDocumentQueue) Ack(ctx context.Context, messageID string) error {
	if strings.TrimSpace(messageID) == "" {
		return errors.New("document queue message id is required")
	}
	if err := q.client.XAck(ctx, q.stream, q.group, messageID).Err(); err != nil {
		return fmt.Errorf("ack document queue message %s: %w", messageID, err)
	}
	return nil
}

func (q *redisDocumentQueue) ClaimStale(ctx context.Context, consumer string, minIdle time.Duration, count int64) ([]Message, error) {
	if strings.TrimSpace(consumer) == "" {
		return nil, errors.New("document queue consumer is required")
	}
	if count <= 0 {
		return nil, errors.New("document queue pending claim count must be positive")
	}
	pageSize := count
	if pageSize < 100 {
		pageSize = 100
	}
	ids := make([]string, 0, count)
	start := "-"
	for int64(len(ids)) < count {
		pending, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream: q.stream,
			Group:  q.group,
			Start:  start,
			End:    "+",
			Count:  pageSize,
		}).Result()
		if err != nil {
			return nil, fmt.Errorf("list stale document queue messages: %w", err)
		}
		if len(pending) == 0 {
			break
		}
		for _, item := range pending {
			if item.Idle >= minIdle {
				ids = append(ids, item.ID)
				if int64(len(ids)) == count {
					break
				}
			}
		}
		if int64(len(pending)) < pageSize {
			break
		}
		start = "(" + pending[len(pending)-1].ID
	}
	if len(ids) == 0 {
		return nil, nil
	}
	claimed, err := q.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   q.stream,
		Group:    q.group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Messages: ids,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("claim stale document queue messages: %w", err)
	}
	messages := make([]Message, 0, len(claimed))
	for _, raw := range claimed {
		message, err := parseMessage(raw)
		if err != nil {
			messages = append(messages, Message{ID: raw.ID, InvalidReason: err.Error()})
			continue
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func parseMessage(raw redis.XMessage) (Message, error) {
	jobID := valueString(raw.Values["job_id"])
	if jobID == "" {
		return Message{}, fmt.Errorf("document queue message %s has no job_id", raw.ID)
	}
	createdAtText := valueString(raw.Values["created_at"])
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return Message{}, fmt.Errorf("document queue message %s has invalid created_at: %w", raw.ID, err)
	}
	schema, err := strconv.Atoi(valueString(raw.Values["schema"]))
	if err != nil || schema <= 0 {
		return Message{}, fmt.Errorf("document queue message %s has invalid schema", raw.ID)
	}
	return Message{
		ID:        raw.ID,
		JobID:     jobID,
		KBID:      valueString(raw.Values["kb_id"]),
		FileID:    valueString(raw.Values["file_id"]),
		CreatedAt: createdAt,
		Schema:    schema,
	}, nil
}

func valueString(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	case nil:
		return ""
	default:
		return fmt.Sprint(value)
	}
}
