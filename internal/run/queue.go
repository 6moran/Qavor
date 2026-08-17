package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"Qavor/internal/trace"

	"github.com/redis/go-redis/v9"
)

// 队列策略
const (
	QueuePolicyEnqueue = "enqueue" // 排队
)

const (
	queueKey         = "qavor:run:queue"            // 全局排队列表（run_id）
	queuedHash       = "qavor:run:queued:%s"        // 排队请求元数据 hash
	threadLock       = "qavor:run:thread:%s:lock"   // 每线程执行锁
	threadPaused     = "qavor:run:thread:%s:paused" // 线程队列暂停标志
	sessionHeartbeat = "qavor:session:%s:heartbeat" // session 心跳（值为 last_heartbeat_time）
)

// QueueItem 排队请求元数据
type QueueItem struct {
	RunID            string             `json:"run_id"`
	ThreadID         string             `json:"thread_id"`
	AgentSlug        string             `json:"agent_slug"`
	RequestID        string             `json:"request_id"`
	Query            string             `json:"query"`
	TraceID          string             `json:"trace_id,omitempty"` // 旧字段，向后兼容（仅 trace_id）
	Trace            trace.TraceCarrier `json:"trace,omitempty"`    // 新字段：完整 TraceCarrier（trace_id/parent_span_id/request_id/sampled）
	Attempt          int                `json:"attempt,omitempty"`
	ResumeFromRunID  string             `json:"resume_from_run_id,omitempty"`
	ResumeFromSpanID string             `json:"resume_from_span_id,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`

	// 审批相关（仅审批/恢复流程填充）
	ApprovalMode string         `json:"approval_mode,omitempty"` // default/always_trust
	ResumeRunID  string         `json:"resume_run_id,omitempty"` // 被恢复的父 Run ID（created_by_run_id 语义）
	CheckpointID string         `json:"checkpoint_id,omitempty"` // 中断时保存的 checkpoint key
	Targets      map[string]any `json:"targets,omitempty"`       // 中断地址 → 审批决定（approve/reject）
}

// RequestQueue 请求队列（Redis list + 每线程锁）
type RequestQueue struct {
	client  *redis.Client
	lockTTL time.Duration
	block   time.Duration
}

// NewRequestQueue 创建请求队列
func NewRequestQueue(client *redis.Client, lockTTL, block time.Duration) *RequestQueue {
	if lockTTL <= 0 {
		lockTTL = 30 * time.Minute
	}
	if block <= 0 {
		block = 5 * time.Second
	}
	return &RequestQueue{client: client, lockTTL: lockTTL, block: block}
}

// Enqueue 入队：写元数据 hash + RPUSH 到队列
func (q *RequestQueue) Enqueue(ctx context.Context, item QueueItem) error {
	if item.RunID == "" {
		return errors.New("queue: run_id is required")
	}
	hKey := fmt.Sprintf(queuedHash, item.RunID)
	fields := map[string]any{
		"run_id":     item.RunID,
		"thread_id":  item.ThreadID,
		"agent_slug": item.AgentSlug,
		"request_id": item.RequestID,
		"query":      item.Query,
		"trace_id":   item.TraceID,
		"created_at": item.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	// 审批/恢复相关字段
	if item.ApprovalMode != "" {
		fields["approval_mode"] = item.ApprovalMode
	}
	if item.ResumeRunID != "" {
		fields["resume_run_id"] = item.ResumeRunID
	}
	if item.CheckpointID != "" {
		fields["checkpoint_id"] = item.CheckpointID
	}
	if len(item.Targets) > 0 {
		targetsJSON, _ := json.Marshal(item.Targets)
		fields["targets"] = string(targetsJSON)
	}
	// 新增 TraceCarrier 字段（非空时写入，兼容旧数据读取）
	if item.Trace.TraceID != "" {
		fields["trace_id_v2"] = item.Trace.TraceID
	}
	if item.Trace.ParentSpanID != "" {
		fields["trace_parent_span_id"] = item.Trace.ParentSpanID
	}
	if item.Trace.RequestID != "" {
		fields["trace_request_id"] = item.Trace.RequestID
	}
	fields["trace_sampled"] = item.Trace.Sampled
	fields["attempt"] = item.Attempt
	fields["resume_from_run_id"] = item.ResumeFromRunID
	fields["resume_from_span_id"] = item.ResumeFromSpanID
	if err := q.client.HSet(ctx, hKey, fields).Err(); err != nil {
		return fmt.Errorf("queue: hset queued: %w", err)
	}
	// hash 随排队状态存活，取出或取消时删除
	if err := q.client.Expire(ctx, hKey, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("queue: expire queued: %w", err)
	}
	if err := q.client.RPush(ctx, queueKey, item.RunID).Err(); err != nil {
		return fmt.Errorf("queue: rpush: %w", err)
	}
	return nil
}

// Dequeue 阻塞出队（BRPOP），返回 run_id
func (q *RequestQueue) Dequeue(ctx context.Context) (string, error) {
	result, err := q.client.BRPop(ctx, q.block, queueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", fmt.Errorf("queue: brpop: %w", err)
	}
	if len(result) < 2 {
		return "", nil
	}
	return result[1], nil
}

// Requeue 重新放回队尾（线程锁未抢到时使用）
func (q *RequestQueue) Requeue(ctx context.Context, runID string) error {
	return q.client.RPush(ctx, queueKey, runID).Err()
}

// GetQueued 读取排队元数据
func (q *RequestQueue) GetQueued(ctx context.Context, runID string) (*QueueItem, error) {
	hKey := fmt.Sprintf(queuedHash, runID)
	val, err := q.client.HGetAll(ctx, hKey).Result()
	if err != nil {
		return nil, fmt.Errorf("queue: hgetall: %w", err)
	}
	if len(val) == 0 {
		return nil, nil
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, val["created_at"])
	attempt, _ := strconv.Atoi(val["attempt"])
	if attempt < 1 {
		attempt = 1
	}
	item := &QueueItem{
		RunID:            val["run_id"],
		ThreadID:         val["thread_id"],
		AgentSlug:        val["agent_slug"],
		RequestID:        val["request_id"],
		Query:            val["query"],
		TraceID:          val["trace_id"],
		Attempt:          attempt,
		ResumeFromRunID:  val["resume_from_run_id"],
		ResumeFromSpanID: val["resume_from_span_id"],
		CreatedAt:        createdAt,
		ApprovalMode:     val["approval_mode"],
		ResumeRunID:      val["resume_run_id"],
		CheckpointID:     val["checkpoint_id"],
	}
	// 恢复 Targets（JSON 字符串 → map[string]any）
	if targetsStr := val["targets"]; targetsStr != "" {
		var targets map[string]any
		if err := json.Unmarshal([]byte(targetsStr), &targets); err == nil && len(targets) > 0 {
			item.Targets = targets
		}
	}
	// 恢复 TraceCarrier：优先读新字段（trace_id_v2），回退到旧 trace_id
	carrier := trace.TraceCarrier{
		TraceID:      val["trace_id_v2"],
		ParentSpanID: val["trace_parent_span_id"],
		RequestID:    val["trace_request_id"],
	}
	if val["trace_sampled"] == "1" {
		carrier.Sampled = true
	}
	// 兼容旧数据：只有 trace_id 时，恢复 TraceID，其余字段为空
	if carrier.TraceID == "" && val["trace_id"] != "" {
		carrier.TraceID = val["trace_id"]
	}
	item.Trace = carrier
	return item, nil
}

// Steer 引导：将排队中的请求移到队首（下一条执行）
func (q *RequestQueue) Steer(ctx context.Context, runID string) error {
	item, err := q.GetQueued(ctx, runID)
	if err != nil {
		return err
	}
	if item == nil {
		return errors.New("queue: request not queued or already running")
	}
	// 先从队列移除，再 LPUSH 到队首
	q.client.LRem(ctx, queueKey, 1, runID)
	if err := q.client.LPush(ctx, queueKey, runID).Err(); err != nil {
		return fmt.Errorf("queue: lpush steer: %w", err)
	}
	return nil
}

// Remove 从队列移除排队请求（取消排队中的请求），返回是否找到
func (q *RequestQueue) Remove(ctx context.Context, runID string) (bool, error) {
	hKey := fmt.Sprintf(queuedHash, runID)
	exists, err := q.client.Exists(ctx, hKey).Result()
	if err != nil {
		return false, fmt.Errorf("queue: exists: %w", err)
	}
	if exists == 0 {
		return false, nil
	}
	q.client.LRem(ctx, queueKey, 0, runID)
	q.client.Del(ctx, hKey)
	return true, nil
}

// ListThread 列出线程的排队请求（按入队顺序）
func (q *RequestQueue) ListThread(ctx context.Context, threadID string) ([]QueueItem, error) {
	// 扫描队列，过滤同线程
	runIDs, err := q.client.LRange(ctx, queueKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("queue: lrange: %w", err)
	}
	items := make([]QueueItem, 0)
	for _, rid := range runIDs {
		item, err := q.GetQueued(ctx, rid)
		if err != nil {
			continue
		}
		if item != nil && item.ThreadID == threadID {
			items = append(items, *item)
		}
	}
	return items, nil
}

// AcquireThreadLock 抢占线程执行锁（SET NX EX），成功返回 true
func (q *RequestQueue) AcquireThreadLock(ctx context.Context, threadID, runID string) (bool, error) {
	key := fmt.Sprintf(threadLock, threadID)
	ok, err := q.client.SetNX(ctx, key, runID, q.lockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("queue: setnx thread lock: %w", err)
	}
	return ok, nil
}

// ReleaseThreadLock 释放线程执行锁（仅当持有者匹配）
func (q *RequestQueue) ReleaseThreadLock(ctx context.Context, threadID, runID string) error {
	key := fmt.Sprintf(threadLock, threadID)
	// 用 Lua 保证只删自己的锁
	script := redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`)
	return script.Run(ctx, q.client, []string{key}, runID).Err()
}

// IsThreadPaused 线程队列是否暂停
func (q *RequestQueue) IsThreadPaused(ctx context.Context, threadID string) (bool, error) {
	key := fmt.Sprintf(threadPaused, threadID)
	v, err := q.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return v > 0, nil
}

// PauseThread / ResumeThread 暂停 / 恢复线程队列
func (q *RequestQueue) PauseThread(ctx context.Context, threadID string) error {
	return q.client.Set(ctx, fmt.Sprintf(threadPaused, threadID), "1", 0).Err()
}

func (q *RequestQueue) ResumeThread(ctx context.Context, threadID string) error {
	return q.client.Del(ctx, fmt.Sprintf(threadPaused, threadID)).Err()
}

// QueueLength 队列长度（调试用）
func (q *RequestQueue) QueueLength(ctx context.Context) (int64, error) {
	v, err := q.client.LLen(ctx, queueKey).Result()
	if err != nil {
		return 0, err
	}
	return v, nil
}

// UpdateSessionHeartbeat 更新 session 心跳时间
func (q *RequestQueue) UpdateSessionHeartbeat(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf(sessionHeartbeat, sessionID)
	return q.client.Set(ctx, key, time.Now().Unix(), q.lockTTL).Err()
}

// IsSessionAlive 检查 session 是否存活（心跳未超时）
func (q *RequestQueue) IsSessionAlive(ctx context.Context, sessionID string) (bool, error) {
	key := fmt.Sprintf(sessionHeartbeat, sessionID)
	exists, err := q.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// CleanSessionHeartbeat 清理 session 心跳
func (q *RequestQueue) CleanSessionHeartbeat(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf(sessionHeartbeat, sessionID)
	return q.client.Del(ctx, key).Err()
}
