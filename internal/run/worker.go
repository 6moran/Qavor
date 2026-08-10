package run

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"Qavor/internal/eventbus"
	longterm "Qavor/internal/memory/long_term"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/trace"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// StreamEvent Agent 执行过程中产生的流式事件（由 AgentExecutor 适配层发出）
type StreamEvent struct {
	Type      string                 // "text_delta" / "tool_call" / "tool_result" / "message_end" / "todo_update"
	MessageID string                 // 聚合同一段输出的 token（同一段输出共享）
	Role      string                 // "assistant" / "tool"
	Content   string                 // 文本内容
	Reasoning string                 // 推理内容增量（reasoning part 文本）
	ToolCall  *eventbus.ToolCallInfo // 工具调用结构化字段
	Todos     []eventbus.TodoItem    // TODO 列表（todo_update 事件携带）
}

// ErrInterrupted 表示 Agent 因工具审批中断
var ErrInterrupted = errors.New("run: agent interrupted for tool approval")

// AgentExecutor 执行 Agent 并通过 emit 回调发出流式事件，返回完整的 Assistant 消息列表（用于持久化）。
// 由 agent 包提供适配实现，解耦 run 与 eino adk。
// opts 支持 WithApprovalMode（审批模式）与 WithResume（审批恢复）。
type AgentExecutor interface {
	Execute(ctx context.Context, slug, query string, history []*schema.Message, emit func(StreamEvent), opts ...ExecuteOption) ([]*schema.Message, error)
	GetModelID(ctx context.Context, slug string) uint
}

// Worker Run 执行器：从队列消费请求，执行 Agent，发布事件到 Redis Stream，持久化消息
type Worker struct {
	queue            *RequestQueue
	pub              *eventbus.Publisher
	runRepo          repository.AgentRunRepository
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	executor         AgentExecutor
	contextMgr       ContextProvider
	longTermMgr      *longterm.Manager
	todoStore        *TodoStore
	logger           *zap.Logger

	// 运行中 Run 的取消函数注册表：run_id -> cancelFunc
	cancels sync.Map

	workerCount int
	block       time.Duration
}

// ContextProvider 上下文管理接口（用于加载对话历史和短期记忆）
type ContextProvider interface {
	LoadHistory(ctx context.Context, conversationID uint) ([]*schema.Message, error)
	UpdateShortMemory(ctx context.Context, conversationID uint, message *schema.Message, modelID uint) error
}

// NewWorker 创建 Run 执行器
func NewWorker(queue *RequestQueue, pub *eventbus.Publisher, runRepo repository.AgentRunRepository,
	messageRepo repository.MessageRepository, conversationRepo repository.ConversationRepository,
	executor AgentExecutor, contextMgr ContextProvider, longTermMgr *longterm.Manager, todoStore *TodoStore,
	logger *zap.Logger, workerCount int) *Worker {
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
		contextMgr:       contextMgr,
		longTermMgr:      longTermMgr,
		todoStore:        todoStore,
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

	// 0.2 恢复 trace 上下文（异步透传：TraceID 来自入队时的 HTTP 请求）
	if item.TraceID != "" && trace.Enabled() {
		ctx = trace.WithTraceContext(ctx, &trace.TraceContext{
			TraceID:        item.TraceID,
			Source:         entity.TraceSourceRun,
			AgentSlug:      item.AgentSlug,
			ConversationID: conversationID,
			RunID:          run.ID,
			RequestID:      requestID,
			Query:          item.Query,
		})
	}

	// 0.05 解析模型 ID（从 Agent 配置中解析，按参数传入短期记忆，避免全局共享）
	modelID := w.executor.GetModelID(ctx, item.AgentSlug)

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

		// 更新短期记忆（用户消息）
		if w.contextMgr != nil {
			userSchemaMsg := &schema.Message{Role: schema.User, Content: item.Query}
			if err := w.contextMgr.UpdateShortMemory(ctx, conversationID, userSchemaMsg, modelID); err != nil {
				w.logger.Warn("更新 Short Memory（用户消息）失败", zap.String("run_id", run.ID), zap.Error(err))
			}
		}
	}

	// 0.2 加载对话历史（在保存用户消息之后，获取包含当前用户消息的完整历史）
	var history []*schema.Message
	if conversationID > 0 && w.contextMgr != nil {
		history, _ = w.contextMgr.LoadHistory(ctx, conversationID)
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
		// todo_update: 持久化到 Redis 并发布 SSE
		if ev.Type == "todo_update" && conversationID > 0 && w.todoStore != nil {
			if err := w.todoStore.SaveTodos(ctx, conversationID, ev.Todos); err != nil {
				w.logger.Warn("保存 TODO 列表失败", zap.String("run_id", run.ID), zap.Error(err))
			}
		}
		_, _ = w.pub.PublishPayload(ctx, eventbus.EventMessage, run.ID, threadID, requestID,
			eventbus.ChunkPayload{
				MessageID: ev.MessageID,
				Type:      ev.Type,
				Role:      ev.Role,
				Content:   ev.Content,
				Reasoning: ev.Reasoning,
				ToolCall:  ev.ToolCall,
				Todos:     ev.Todos,
			})
	}

	// 执行选项：审批模式 + 恢复参数（resume 流程）
	var execOpts []ExecuteOption
	if item.ApprovalMode != "" {
		execOpts = append(execOpts, WithApprovalMode(item.ApprovalMode))
	}
	if item.ResumeRunID != "" && item.CheckpointID != "" {
		execOpts = append(execOpts, WithResume(item.CheckpointID, item.Targets))
	}

	assistantMsgs, execErr := w.executor.Execute(ctx, item.AgentSlug, item.Query, history, emit, execOpts...)

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
			// 更新短期记忆（助手消息）
			if w.contextMgr != nil {
				if err := w.contextMgr.UpdateShortMemory(ctx, conversationID, &schema.Message{
					Role:    schema.Assistant,
					Content: msg.Content,
				}, modelID); err != nil {
					w.logger.Warn("更新 Short Memory（助手消息）失败", zap.String("run_id", run.ID), zap.Error(err))
				}
			}
		}
	}

	// 3.2 异步抽取长期记忆（跨会话画像/偏好/决策）——失败不影响主流程
	if conversationID > 0 && w.longTermMgr != nil &&
		item.Query != "" && len(assistantMsgs) > 0 && assistantMsgs[len(assistantMsgs)-1] != nil {
		var lastAssistantContent string
		for i := len(assistantMsgs) - 1; i >= 0; i-- {
			if assistantMsgs[i] != nil && assistantMsgs[i].Content != "" {
				lastAssistantContent = assistantMsgs[i].Content
				break
			}
		}
		if lastAssistantContent != "" {
			w.longTermMgr.ExtractAfterTurn(ctx, 0, conversationID, run.ID, []longterm.TurnMessage{
				{Role: "user", Content: item.Query},
				{Role: "assistant", Content: lastAssistantContent},
			}, modelID)
		}
	}

	// 4. 根据结果发布终态事件并收尾 trace
	switch {
	case ctx.Err() == context.Canceled:
		trace.FinishTrace(ctx, entity.TraceStatusCancelled, "context cancelled")
		w.finish(ctx, run, eventbus.StatusCancelled, "cancelled")
	case errors.Is(execErr, ErrInterrupted):
		trace.FinishTrace(ctx, entity.TraceStatusCancelled, "interrupted")
		// 提取审批信息并发布中断+审批事件
		if ie, ok := execErr.(*InterruptedError); ok {
			w.publishApproval(ctx, run, ie)
		}
		// 保存中断信息到 run 记录
		if ie, ok := execErr.(*InterruptedError); ok {
			w.saveInterruptInfo(ctx, run, ie)
		}
		w.finish(ctx, run, eventbus.StatusInterrupted, "interrupted")
	case execErr != nil:
		w.logger.Error("Run 执行失败",
			zap.String("run_id", run.ID),
			zap.String("agent_slug", item.AgentSlug),
			zap.String("error", execErr.Error()),
			zap.Int("assistant_msgs_count", len(assistantMsgs)),
		)
		trace.FinishTrace(ctx, entity.TraceStatusFailed, execErr.Error())
		w.publishError(ctx, run, execErr)
		w.finish(ctx, run, eventbus.StatusFailed, "failed")
	default:
		trace.FinishTrace(ctx, entity.TraceStatusSuccess, "")
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
	code := "AGENT_ERROR"
	msg := execErr.Error()

	// 识别 LLM 额度用尽等常见错误，返回友好提示
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "free quota exhausted"),
		strings.Contains(lower, "quota exhausted"),
		strings.Contains(lower, "额度"):
		code = "QUOTA_EXHAUSTED"
		msg = "模型额度已用完，请充值或更换模型"
	case strings.Contains(lower, "rate limit"),
		strings.Contains(lower, "429"),
		strings.Contains(lower, "too many requests"):
		code = "RATE_LIMITED"
		msg = "请求过于频繁，请稍后重试"
	case strings.Contains(lower, "invalid api key"),
		strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "authentication"):
		code = "AUTH_ERROR"
		msg = "模型认证失败，请检查 API Key 配置"
	}

	_, _ = w.pub.PublishPayload(ctx, eventbus.EventError, run.ID, run.ConversationThreadID, run.RequestID,
		eventbus.ErrorPayload{Code: code, Message: msg})
}

// publishApproval 发布审批请求事件。
// 前端 useApproval.js 的 processApprovalInStream 检查 chunk.status === 'human_approval_required'，
// 该状态只由 event: message 的 items 触发，因此先发一条 message 事件携带审批数据，
// 再发 end 事件标记中断终态（前端 SSE 流在此结束）。
// approval.action_requests 与 approval.review_configs 长度必须相等（前端 useApproval.js:27 强校验）。
func (w *Worker) publishApproval(ctx context.Context, run *entity.AgentRun, ie *InterruptedError) {
	approval := &eventbus.ApprovalPayload{}
	for _, req := range ie.Requests {
		approval.ActionRequests = append(approval.ActionRequests, eventbus.ApprovalActionRequest{
			Name: req.ToolName,
			Args: req.Args,
		})
		// review_configs 与 action_requests 一一对应，前端要求长度相等
		approval.ReviewConfigs = append(approval.ReviewConfigs, eventbus.ApprovalReviewConfig{
			ToolName: req.ToolName,
			Args:     req.Args,
			Reason:   "此操作需要用户审批",
		})
	}

	// 1. 先发 message 事件：items 中包含审批 chunk，触发前端审批弹窗。
	// 前端 processRunSseResponse 在 event: message + payload.items 时，
	// 将每个 item 直接作为 chunk 传给 handleStreamChunk，
	// handleStreamChunk 的 case 'human_approval_required' 分支会提取 approval 并弹窗。
	msgPayload := map[string]any{
		"items": []map[string]any{
			{
				"status":   "human_approval_required",
				"approval": approval,
				"run_id":   run.ID,
				"thread_id": run.ConversationThreadID,
			},
		},
	}
	_, _ = w.pub.PublishPayload(ctx, eventbus.EventMessage, run.ID, run.ConversationThreadID, run.RequestID, msgPayload)

	// 2. 再发 end 事件标记中断终态。
	_, _ = w.pub.PublishPayload(ctx, eventbus.EventEnd, run.ID, run.ConversationThreadID, run.RequestID,
		eventbus.EndPayload{
			Status:   eventbus.StatusInterrupted,
			Approval: approval,
		})
}

// saveInterruptInfo 保存中断信息到 run 记录（checkpointID + approval info），
// 供 resume 时读取。
func (w *Worker) saveInterruptInfo(_ context.Context, run *entity.AgentRun, ie *InterruptedError) {
	run.CheckpointID = ie.CheckpointID
	// ApprovalInfo 与 EndPayload 的 approval 结构一致（前端解析用）
	run.ApprovalInfo = entity.JSON{
		"action_requests": ie.Requests,
		"checkpoint_id":   ie.CheckpointID,
	}
	if err := w.runRepo.Update(run); err != nil {
		w.logger.Warn("worker 保存中断信息失败", zap.String("run_id", run.ID), zap.Error(err))
	}
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
