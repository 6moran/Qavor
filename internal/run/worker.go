package run

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"Qavor/internal/eventbus"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// StreamEvent Agent 执行过程中产生的流式事件（由 AgentExecutor 适配层发出）
type StreamEvent struct {
	Type      string                 // "text_delta" / "tool_call" / "tool_result" / "message_end"
	MessageID string                 // 聚合同一段输出的 token（同一段输出共享）
	Role      string                 // "assistant" / "tool"
	Content   string                 // 文本内容
	ToolCall  *eventbus.ToolCallInfo // 工具调用结构化字段
}

// ErrInterrupted 表示 Agent 因工具审批中断
var ErrInterrupted = errors.New("run: agent interrupted for tool approval")

// AgentExecutor 执行 Agent 并通过 emit 回调发出流式事件，返回完整的 Assistant 消息列表（用于持久化）。
// 由 agent 包提供适配实现，解耦 run 与 eino adk。
type AgentExecutor interface {
	Execute(ctx context.Context, slug, query string, emit func(StreamEvent)) ([]*schema.Message, error)
}

// Worker Run 执行器：从队列消费请求，执行 Agent，发布事件到 Redis Stream，持久化消息
type Worker struct {
	queue            *RequestQueue
	pub              *eventbus.Publisher
	runRepo          repository.AgentRunRepository
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	executor         AgentExecutor
	logger           *zap.Logger

	// 运行中 Run 的取消函数注册表：run_id -> cancelFunc
	cancels sync.Map

	workerCount int
	block       time.Duration
}

// NewWorker 创建 Run 执行器
func NewWorker(queue *RequestQueue, pub *eventbus.Publisher, runRepo repository.AgentRunRepository,
	messageRepo repository.MessageRepository, conversationRepo repository.ConversationRepository,
	executor AgentExecutor, logger *zap.Logger, workerCount int) *Worker {
	if workerCount <= 0 {
		workerCount = 3
	}
	return &Worker{
		queue:            queue,
		pub:              pub,
		runRepo:          runRepo,
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		executor:         executor,
		logger:           logger,
		workerCount:      workerCount,
		block:            5 * time.Second,
	}
}

// Run 启动 worker 池，阻塞直到 ctx 取消
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.workerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w.loop(ctx, id)
		}(i)
	}
	wg.Wait()
}

func (w *Worker) loop(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		runID, err := w.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Warn("worker dequeue 失败", zap.Int("worker", id), zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		if runID == "" {
			continue
		}
		w.process(ctx, runID)
	}
}

// process 处理单个 Run
func (w *Worker) process(parent context.Context, runID string) {
	item, err := w.queue.GetQueued(parent, runID)
	if err != nil || item == nil {
		// 元数据丢失（可能已取消或过期），跳过
		if err != nil {
			w.logger.Warn("worker 读取排队元数据失败", zap.String("run_id", runID), zap.Error(err))
		}
		return
	}

	// 暂停的线程：放回队尾等待恢复
	paused, _ := w.queue.IsThreadPaused(parent, item.ThreadID)
	if paused {
		_ = w.queue.Requeue(parent, runID)
		time.Sleep(200 * time.Millisecond)
		return
	}

	// 抢占线程锁（每线程同时只跑一个 Run）
	acquired, err := w.queue.AcquireThreadLock(parent, item.ThreadID, runID)
	if err != nil {
		w.logger.Warn("worker 抢占线程锁失败", zap.String("run_id", runID), zap.Error(err))
		_ = w.queue.Requeue(parent, runID)
		return
	}
	if !acquired {
		// 线程已有活跃 Run，放回队尾稍后重试
		_ = w.queue.Requeue(parent, runID)
		time.Sleep(200 * time.Millisecond)
		return
	}
	defer func() {
		if err := w.queue.ReleaseThreadLock(parent, item.ThreadID, runID); err != nil {
			w.logger.Warn("worker 释放线程锁失败", zap.String("run_id", runID), zap.Error(err))
		}
	}()

	// 移除排队元数据
	_, _ = w.queue.Remove(parent, runID)

	// 加载 Run 记录，校验状态
	run, err := w.runRepo.GetByID(runID)
	if err != nil || run == nil {
		w.logger.Error("worker 加载 Run 失败", zap.String("run_id", runID), zap.Error(err))
		return
	}
	if run.IsTerminal() {
		return
	}

	// 带 cancel 的执行 ctx，注册到取消表
	ctx, cancel := context.WithCancel(parent)
	w.cancels.Store(runID, cancel)
	defer func() {
		w.cancels.Delete(runID)
		cancel()
	}()

	w.execute(ctx, run, item)
}

// execute 执行 Agent 并发布事件
func (w *Worker) execute(ctx context.Context, run *entity.AgentRun, item *QueueItem) {
	threadID := run.ConversationThreadID
	requestID := run.RequestID

	// 0. 解析 conversation_id（用于消息持久化）
	// 前端可能传 Conversation.ThreadID（UUID）或 Conversation.ID（数字），两者都兼容
	var conversationID uint
	if conv, err := w.conversationRepo.FindByThreadID(threadID); err == nil && conv != nil {
		conversationID = conv.ID
	} else if numID, parseErr := strconv.ParseUint(threadID, 10, 32); parseErr == nil {
		if conv, err := w.conversationRepo.FindByID(uint(numID)); err == nil && conv != nil {
			conversationID = conv.ID
		}
	}
	if conversationID == 0 {
		w.logger.Warn("worker 无法解析 conversation_id，消息将不会被持久化",
			zap.String("run_id", run.ID), zap.String("thread_id", threadID))
	}

	// 0.1 保存用户消息
	if conversationID > 0 && item.Query != "" {
		userMsg := &entity.Message{
			ConversationID: conversationID,
			Role:           "user",
			Content:        item.Query,
			RunID:          run.ID,
			RequestID:      requestID,
		}
		if err := w.messageRepo.Create(userMsg); err != nil {
			w.logger.Warn("worker 保存用户消息失败", zap.String("run_id", run.ID), zap.Error(err))
		}
	}

	// 1. 状态置 running
	if err := w.runRepo.UpdateStatus(run.ID, entity.StatusRunning, ""); err != nil {
		w.logger.Warn("worker 更新 running 状态失败", zap.String("run_id", run.ID), zap.Error(err))
	}

	// 2. 发布 metadata 事件
	if _, err := w.pub.PublishPayload(ctx, eventbus.EventMetadata, run.ID, threadID, requestID,
		eventbus.MetadataPayload{RunType: run.RunType, Source: "chat"}); err != nil {
		w.logger.Warn("worker 发布 metadata 失败", zap.String("run_id", run.ID), zap.Error(err))
	}

	// 3. 执行 Agent，emit 回调发布 message 事件
	emit := func(ev StreamEvent) {
		if ctx.Err() != nil {
			return
		}
		_, _ = w.pub.PublishPayload(ctx, eventbus.EventMessage, run.ID, threadID, requestID,
			eventbus.ChunkPayload{
				MessageID: ev.MessageID,
				Type:      ev.Type,
				Role:      ev.Role,
				Content:   ev.Content,
				ToolCall:  ev.ToolCall,
			})
	}

	assistantMsgs, execErr := w.executor.Execute(ctx, item.AgentSlug, item.Query, emit)

	// 3.1 持久化 Assistant 消息（刷新后可从 DB 加载）—— 无论成功或失败都保存已生成的消息
	if conversationID > 0 && len(assistantMsgs) > 0 {
		for _, msg := range assistantMsgs {
			if msg == nil || msg.Content == "" {
				continue
			}
			aiMsg := &entity.Message{
				ConversationID: conversationID,
				Role:           "assistant",
				Content:        msg.Content,
				RunID:          run.ID,
				RequestID:      requestID,
			}
			if err := w.messageRepo.Create(aiMsg); err != nil {
				w.logger.Error("worker 保存 AI 消息失败", zap.String("run_id", run.ID), zap.Error(err))
			}
		}
	}

	// 4. 根据结果发布终态事件
	switch {
	case ctx.Err() == context.Canceled:
		w.finish(ctx, run, eventbus.StatusCancelled, "cancelled")
	case errors.Is(execErr, ErrInterrupted):
		w.finish(ctx, run, eventbus.StatusInterrupted, "interrupted")
	case execErr != nil:
		w.logger.Error("Run 执行失败",
			zap.String("run_id", run.ID),
			zap.String("agent_slug", item.AgentSlug),
			zap.String("error", execErr.Error()),
			zap.Int("assistant_msgs_count", len(assistantMsgs)),
		)
		w.publishError(ctx, run, execErr)
		w.finish(ctx, run, eventbus.StatusFailed, "failed")
	default:
		w.finish(ctx, run, eventbus.StatusCompleted, "completed")
	}
}

// finish 发布 end 事件并更新终态
func (w *Worker) finish(ctx context.Context, run *entity.AgentRun, status, repoStatus string) {
	// 先发布 end 事件
	var endID string
	if id, err := w.pub.PublishPayload(ctx, eventbus.EventEnd, run.ID, run.ConversationThreadID, run.RequestID,
		eventbus.EndPayload{Status: status}); err == nil {
		endID = id
	}
	// 再更新 DB 状态（last_event_id 为 end 事件 ID）
	if err := w.runRepo.UpdateStatus(run.ID, repoStatus, endID); err != nil {
		w.logger.Warn("worker 更新终态失败", zap.String("run_id", run.ID), zap.Error(err))
	}
}

// publishError 发布 error 事件
func (w *Worker) publishError(ctx context.Context, run *entity.AgentRun, execErr error) {
	_, _ = w.pub.PublishPayload(ctx, eventbus.EventError, run.ID, run.ConversationThreadID, run.RequestID,
		eventbus.ErrorPayload{Code: "AGENT_ERROR", Message: execErr.Error()})
}

// CancelRun 取消运行中的 Run（调用其 cancel 函数）。返回是否找到运行中的 Run。
func (w *Worker) CancelRun(runID string) bool {
	v, ok := w.cancels.Load(runID)
	if !ok {
		return false
	}
	if cancel, ok := v.(context.CancelFunc); ok {
		cancel()
		return true
	}
	return false
}
