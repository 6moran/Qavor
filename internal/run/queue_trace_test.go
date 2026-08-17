package run

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"Qavor/internal/trace"

	"github.com/redis/go-redis/v9"
)

// newTestQueue 创建测试用 Redis 队列。
// 使用 QAVOR_TEST_REDIS 指定地址，并强制使用非 0 的专用测试 DB；
// 未设置时 Skip 测试（不假通过），避免误清理开发数据。
func newTestQueue(t *testing.T) *RequestQueue {
	t.Helper()
	addr := os.Getenv("QAVOR_TEST_REDIS")
	if addr == "" {
		t.Skipf("跳过 Redis 集成测试：设置 QAVOR_TEST_REDIS=<addr> 以启用（如 localhost:6379）")
	}
	db, err := strconv.Atoi(os.Getenv("QAVOR_TEST_REDIS_DB"))
	if err != nil || db <= 0 {
		t.Fatalf("拒绝使用 Redis DB 0 或无效 DB：请设置 QAVOR_TEST_REDIS_DB 为专用测试 DB（> 0）")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: db})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("连接测试 Redis 失败: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return &RequestQueue{client: client, lockTTL: time.Minute, block: 100 * time.Millisecond}
}

func TestQueueItemTraceCarrierRoundTrip(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	want := trace.TraceCarrier{
		TraceID:      "11111111-1111-1111-1111-111111111111",
		ParentSpanID: "http-span-1",
		RequestID:    "req-1",
		Sampled:      true,
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	item := QueueItem{
		RunID:     "run-carrier-1",
		ThreadID:  "thread-1",
		AgentSlug: "assistant",
		RequestID: "req-1",
		Query:     "hello",
		Trace:     want,
		CreatedAt: now,
	}
	if err := q.Enqueue(ctx, item); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetQueued(ctx, item.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("GetQueued returned nil")
	}
	if got.Trace != want {
		t.Fatalf("trace carrier round-trip mismatch:\n got  = %+v\n want = %+v", got.Trace, want)
	}
	// 清理
	_, _ = q.Remove(ctx, item.RunID)
}

func TestQueueItemLegacyTraceIDCompat(t *testing.T) {
	// 旧数据只有 trace_id 字段（无 trace_id_v2），恢复后 TraceID 从 trace_id 读取
	q := newTestQueue(t)
	ctx := context.Background()

	hKey := "qavor:run:queued:run-legacy-1"
	// 手动写入旧格式数据（只有 trace_id，没有 trace_id_v2 等）
	if err := q.client.HSet(ctx, hKey, map[string]any{
		"run_id":     "run-legacy-1",
		"thread_id":  "thread-1",
		"agent_slug": "assistant",
		"request_id": "req-legacy",
		"query":      "legacy",
		"trace_id":   "old-trace-id",
		"created_at": time.Now().UTC().Format(time.RFC3339Nano),
	}).Err(); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetQueued(ctx, "run-legacy-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("GetQueued returned nil for legacy data")
	}
	if got.Trace.TraceID != "old-trace-id" {
		t.Fatalf("legacy trace_id = %q, want old-trace-id", got.Trace.TraceID)
	}
	if got.Trace.ParentSpanID != "" {
		t.Fatalf("legacy parent_span_id = %q, want empty", got.Trace.ParentSpanID)
	}
	// 清理
	q.client.Del(ctx, hKey)
}

func TestQueueItemJSONSerializationCarrierRoundTrip(t *testing.T) {
	// 不依赖 Redis：验证 QueueItem 的 JSON 序列化/反序列化保留 TraceCarrier
	want := trace.TraceCarrier{
		TraceID:      "22222222-2222-2222-2222-222222222222",
		ParentSpanID: "parent-span-2",
		RequestID:    "req-2",
		Sampled:      true,
	}
	item := QueueItem{
		RunID: "run-json-1",
		Trace: want,
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var got QueueItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Trace != want {
		t.Fatalf("JSON round-trip mismatch:\n got  = %+v\n want = %+v", got.Trace, want)
	}
}

func TestQueueItemJSONSerializationResumeMetadata(t *testing.T) {
	want := QueueItem{
		RunID:            "run-resume-2",
		Attempt:          3,
		ResumeFromRunID:  "run-interrupted-1",
		ResumeFromSpanID: "agent-span-interrupted",
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got QueueItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Attempt != want.Attempt || got.ResumeFromRunID != want.ResumeFromRunID || got.ResumeFromSpanID != want.ResumeFromSpanID {
		t.Fatalf("resume metadata = %+v, want %+v", got, want)
	}
}
