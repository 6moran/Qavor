package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"Qavor/internal/eventbus"
	"Qavor/internal/llm"
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

// toolExecutionResult 保存一次工具调用的实际执行结果。
// Worker 会先收到工具结果事件，稍后再把它回填到 AI 消息中的对应 ToolCall。
type toolExecutionResult struct {
	Output       string
	Status       string
	ErrorMessage string
}

// recordToolExecutionResult 按 ToolCallID 暂存工具结果。
// 不能按工具名称关联，因为同一轮对话中可能多次调用同一个工具；缺少调用 ID 的事件
// 无法可靠匹配，因此直接忽略，避免把知识库结果写到错误的工具调用上。
func recordToolExecutionResult(results map[string]toolExecutionResult, ev StreamEvent) {
	if ev.Type != "tool_result" || ev.ToolCall == nil || ev.ToolCall.ID == "" {
		return
	}
	results[ev.ToolCall.ID] = toolExecutionResult{
		Output: ev.Content,
		Status: "success",
	}
}

// buildPersistedToolCalls 将模型发起的工具调用与执行阶段收集到的结果合并为数据库实体。
// 尚未收到对应结果的调用保持 pending，避免像旧逻辑一样无条件标记 success，
// 同时保留空输出，便于后续区分“未执行完成”和“执行成功但没有返回内容”。
func buildPersistedToolCalls(toolCalls []schema.ToolCall, results map[string]toolExecutionResult) []entity.ToolCall {
	persisted := make([]entity.ToolCall, 0, len(toolCalls))
	for i := range toolCalls {
		tc := &toolCalls[i]
		var toolInput entity.JSON
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &toolInput)
		}

		call := entity.ToolCall{
			LanggraphToolCallID: tc.ID,
			ToolName:            tc.Function.Name,
			ToolInput:           toolInput,
			Status:              "pending",
		}
		if result, ok := results[tc.ID]; ok {
			call.ToolOutput = result.Output
			call.Status = result.Status
			call.ErrorMessage = result.ErrorMessage
		}
		persisted = append(persisted, call)
	}
	return persisted
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

// EventPublisher 是 Worker 对事件总线的最小依赖，便于隔离发布故障并保证业务终态不受影响。
type EventPublisher interface {
	PublishPayload(ctx context.Context, eventType, runID, threadID, requestID string, payload any) (string, error)
}

// Worker Run 执行器：从队列消费请求，执行 Agent，发布事件到 Redis Stream，持久化消息
type Worker struct {
	queue            *RequestQueue
	pub              EventPublisher
	runRepo          repository.AgentRunRepository
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	executor         AgentExecutor
	contextMgr       ContextProvider
	longTermMgr      *longterm.Manager
	todoStore        *TodoStore
	modelSvc         ModelResolver // 用于获取模型信息以动态调整上下文窗口
	logger           *zap.Logger
	tracer           *trace.Tracer

	// 运行中 Run 的取消函数注册表：run_id -> cancelFunc
	cancels sync.Map

	workerCount int
	block       time.Duration
}

// ContextProvider 上下文管理接口（用于加载对话历史和短期记忆）
type ContextProvider interface {
	// LoadHistory 加载对话历史，maxTokens 为该请求对应的模型上下文窗口大小，modelID 用于摘要生成
	LoadHistory(ctx context.Context, conversationID uint, maxTokens int, modelID uint) ([]*schema.Message, error)
	UpdateShortMemory(ctx context.Context, conversationID uint, message *schema.Message, modelID uint) error
}

func queueRunMeta(run *entity.AgentRun, item *QueueItem, conversationID uint, requestID string) trace.RunMeta {
	attempt := item.Attempt
	if attempt < 1 {
		attempt = 1
	}
	return trace.RunMeta{
		RunID:            run.ID,
		AgentSlug:        item.AgentSlug,
		ConversationID:   conversationID,
		RequestID:        requestID,
		Query:            item.Query,
		Mode:             "async",
		Attempt:          attempt,
		ResumeFromRunID:  item.ResumeFromRunID,
		ResumeFromSpanID: item.ResumeFromSpanID,
	}
}

// NewWorker 创建 Run 执行器。
func NewWorker(queue *RequestQueue, pub EventPublisher, runRepo repository.AgentRunRepository,
	messageRepo repository.MessageRepository, conversationRepo repository.ConversationRepository,
	executor AgentExecutor, contextMgr ContextProvider, longTermMgr *longterm.Manager, todoStore *TodoStore,
	modelSvc ModelResolver, logger *zap.Logger, workerCount int,
	tracer *trace.Tracer) *Worker {
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
		modelSvc:         modelSvc,
		logger:           logger,
		workerCount:      workerCount,
		block:            5 * time.Second,
		tracer:           tracer,
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
	if conv, err := w.conversationRepo.FindByThreadID(ctx, threadID); err == nil && conv != nil {
		conversationID = conv.ID
	} else if numID, parseErr := strconv.ParseUint(threadID, 10, 32); parseErr == nil {
		if conv, err := w.conversationRepo.FindByID(ctx, uint(numID)); err == nil && conv != nil {
			conversationID = conv.ID
		}
	}
	if conversationID == 0 {
		w.logger.Error("worker 无法解析 conversation_id，消息将不会被持久化",
			zap.String("run_id", run.ID), zap.String("thread_id", threadID))
		// 尝试通过 AgentRun 记录查找 conversation
		if run.ConversationThreadID != "" {
			w.logger.Info("尝试通过 run.ConversationThreadID 查找 conversation",
				zap.String("run_id", run.ID), zap.String("conversation_thread_id", run.ConversationThreadID))
		}
	}

	// 0.2 恢复 trace 上下文：从 TraceCarrier 恢复父 SpanContext，创建 queue.consume Span，
	// 注入 RunMeta 供 AgentRuntime 读取。consumeSpan 从开始处理覆盖到终态发布。
	var consumeSpan *trace.Span
	var execErr error // 记录最终执行结果，供 defer 结束 consumeSpan
	var assistantMsgs []*schema.Message
	if w.tracer != nil && item.Trace.TraceID != "" {
		ctx = trace.ContextFromCarrier(ctx, item.Trace)
		queueWaitMs := time.Since(item.CreatedAt).Milliseconds()
		if queueWaitMs < 0 {
			w.logger.Warn("queue_wait_ms 为负数，可能存在时钟漂移",
				zap.String("run_id", run.ID),
				zap.Time("created_at", item.CreatedAt),
				zap.Int64("queue_wait_ms", queueWaitMs),
			)
			queueWaitMs = 0
		}
		ctx, consumeSpan = w.tracer.StartSpan(ctx, trace.SpanSpec{
			Kind:      "queue",
			Operation: "queue.consume",
			RunID:     run.ID,
			RequestID: requestID,
			Attributes: entity.JSON{
				"queue":         "qavor:run:queue",
				"queue_wait_ms": queueWaitMs,
			},
		})
	}
	// RunMeta 属于业务执行上下文，不依赖 Trace 是否开启或旧队列数据是否带完整 carrier。
	ctx = trace.WithRunMeta(ctx, queueRunMeta(run, item, conversationID, requestID))
	defer func() {
		if consumeSpan == nil {
			return
		}
		if recovered := recover(); recovered != nil {
			consumeSpan.End(trace.SpanEnd{
				Status:       trace.SpanStatusError,
				ErrorType:    "panic",
				ErrorMessage: fmt.Sprint(recovered),
			})
			panic(recovered) // 不吞掉 panic
		}
		// 统一根据 execErr 和 ctx 状态结束 consumeSpan，禁止在 switch 分支复制 End 调用
		if ctx.Err() != nil {
			consumeSpan.End(trace.SpanEnd{
				Status:       trace.SpanStatusCancelled,
				ErrorMessage: ctx.Err().Error(),
			})
			return
		}
		if execErr == nil {
			consumeSpan.End(trace.SpanEnd{Status: trace.SpanStatusOK})
			return
		}
		if errors.Is(execErr, ErrInterrupted) {
			consumeSpan.End(trace.SpanEnd{
				Status:       trace.SpanStatusInterrupted,
				ErrorMessage: execErr.Error(),
			})
			return
		}
		if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) {
			consumeSpan.End(trace.SpanEnd{
				Status:       trace.SpanStatusCancelled,
				ErrorMessage: execErr.Error(),
			})
			return
		}
		consumeSpan.End(trace.SpanEnd{
			Status:       trace.SpanStatusError,
			ErrorMessage: execErr.Error(),
		})
	}()

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
		// 根据模型动态调整上下文窗口大小
		// 优先级：模型配置 > Provider 默认值 > 全局默认值
		maxTokens := 0
		if w.modelSvc != nil && modelID > 0 {
			if provider, name, contextWindow, ok := w.modelSvc.GetModelInfo(modelID); ok {
				if contextWindow > 0 {
					// 使用模型配置的 ContextWindow
					maxTokens = contextWindow
				} else {
					// 数据库中未配置，使用 Provider 默认值作为后备
					maxTokens = llm.GetMaxContextTokens(provider, name)
				}
			}
		}
		// 传入 modelID 用于摘要生成
		history, _ = w.contextMgr.LoadHistory(ctx, conversationID, maxTokens, modelID)
	}

	// 1. 状态置 running
	if err := w.runRepo.UpdateStatus(ctx, run.ID, entity.StatusRunning, ""); err != nil {
		w.logger.Warn("worker 更新 running 状态失败", zap.String("run_id", run.ID), zap.Error(err))
	}

	// 2. 发布 metadata 事件
	if _, err := w.pub.PublishPayload(ctx, eventbus.EventMetadata, run.ID, threadID, requestID,
		eventbus.MetadataPayload{RunType: run.RunType, Source: "chat"}); err != nil {
		w.logger.Warn("worker 发布 metadata 失败", zap.String("run_id", run.ID), zap.Error(err))
	}

	// 3. 执行 Agent，emit 回调发布 message 事件
	// 工具调用和工具结果来自不同的流事件，先按调用 ID 收集结果，
	// 保存最终 AI 消息时再一次性合并到 tool_calls 中。
	toolResults := make(map[string]toolExecutionResult)
	emit := func(ev StreamEvent) {
		if ctx.Err() != nil {
			return
		}
		recordToolExecutionResult(toolResults, ev)
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

	assistantMsgs, execErr = w.executor.Execute(ctx, item.AgentSlug, item.Query, history, emit, execOpts...)

	// 3.1 持久化 Assistant 消息（刷新后可从 DB 加载）—— 无论成功或失败都保存已生成的消息
	// 使用单个 message.persist Span 覆盖批量持久化，与 agent.run 平级（parent 为 queue.consume）
	if conversationID > 0 && len(assistantMsgs) > 0 {
		var persistSpan *trace.Span
		if w.tracer != nil && consumeSpan != nil {
			_, persistSpan = w.tracer.StartSpan(ctx, trace.SpanSpec{
				Kind:      "persistence",
				Operation: "message.persist",
				RunID:     run.ID,
				RequestID: requestID,
				Attributes: entity.JSON{
					"message_count": len(assistantMsgs),
				},
			})
		}
		persistFailed := false
		for _, msg := range assistantMsgs {
			if msg == nil || (msg.Content == "" && len(msg.ToolCalls) == 0) {
				continue
			}
			reasoningContent := msg.ReasoningContent
			if reasoningContent == "" {
				reasoningContent = extractReasoningFromSchemaMsg(msg)
			}
			aiMsg := &entity.Message{
				ConversationID:   conversationID,
				Role:             "assistant",
				Content:          msg.Content,
				ReasoningContent: reasoningContent,
				RunID:            run.ID,
				RequestID:        requestID,
				IsFinished:       ctx.Err() == nil, // ctx 取消时标记为未完成
			}
			aiMsg.ToolCalls = buildPersistedToolCalls(msg.ToolCalls, toolResults)
			// aiMsg 已通过 aiMsg.ToolCalls 携带 tool_calls，由 Create 显式保存
			if err := w.messageRepo.Create(aiMsg); err != nil {
				w.logger.Error("worker 保存 AI 消息失败", zap.String("run_id", run.ID), zap.Error(err))
				persistFailed = true
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
		if persistSpan != nil {
			if persistFailed {
				persistSpan.End(trace.SpanEnd{
					Status:    trace.SpanStatusError,
					ErrorType: "message_persist",
				})
			} else {
				persistSpan.End(trace.SpanEnd{Status: trace.SpanStatusOK})
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

	// 4. 根据结果发布终态事件（trace 由 defer 统一收尾，不在此处手动结束）
	switch {
	case ctx.Err() == context.Canceled:
		w.finish(ctx, run, eventbus.StatusCancelled, "cancelled")
	case errors.Is(execErr, ErrInterrupted):
		// 提取审批信息并发布中断+审批事件
		if ie, ok := execErr.(*InterruptedError); ok {
			if ie.IsQuestion {
				w.publishQuestion(ctx, run, ie)
			} else {
				w.publishApproval(ctx, run, ie)
			}
		}
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
		w.publishError(ctx, run, execErr)
		w.finish(ctx, run, eventbus.StatusFailed, "failed")
	default:
		w.finish(ctx, run, eventbus.StatusCompleted, "completed")
	}
}

// finish 发布 end 事件并更新终态。
// 创建 event.publish Span 记录终态事件发布，与 agent.run 平级（parent 为 queue.consume）。
// 注意：终态事件发布和 DB 更新使用独立 ctx（带 5s 超时），避免 ctx 被取消时 end 事件丢失导致 SSE 消费端无限等待。
func (w *Worker) finish(ctx context.Context, run *entity.AgentRun, status, repoStatus string) {
	finalCtx, finalCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finalCancel()

	var pubSpan *trace.Span
	if w.tracer != nil {
		_, pubSpan = w.tracer.StartSpan(ctx, trace.SpanSpec{
			Kind:      "event",
			Operation: "event.publish",
			RunID:     run.ID,
			RequestID: run.RequestID,
			Attributes: entity.JSON{
				"event_type": eventbus.EventEnd,
				"run_id":     run.ID,
				"status":     status,
			},
		})
	}
	// 先发布 end 事件
	var endID string
	var publishErr error
	if id, err := w.pub.PublishPayload(finalCtx, eventbus.EventEnd, run.ID, run.ConversationThreadID, run.RequestID,
		eventbus.EndPayload{Status: status}); err == nil {
		endID = id
	} else {
		publishErr = err
		w.logger.Warn("worker 发布终态事件失败", zap.String("run_id", run.ID), zap.Error(err))
		if pubSpan != nil {
			pubSpan.End(trace.SpanEnd{
				Status:       trace.SpanStatusError,
				ErrorType:    "event_publish",
				ErrorMessage: err.Error(),
			})
		}
	}
	// 再更新 DB 状态（last_event_id 为 end 事件 ID）
	if err := w.runRepo.UpdateStatus(finalCtx, run.ID, repoStatus, endID); err != nil {
		w.logger.Warn("worker 更新终态失败", zap.String("run_id", run.ID), zap.Error(err))
		if pubSpan != nil && publishErr == nil {
			pubSpan.End(trace.SpanEnd{
				Status:       trace.SpanStatusError,
				ErrorType:    "run_status_update",
				ErrorMessage: err.Error(),
			})
			return
		}
	}
	if pubSpan != nil && publishErr == nil {
		pubSpan.End(trace.SpanEnd{Status: trace.SpanStatusOK})
	}
}

// publishError 发布 error 事件
// 注意：error 事件发布使用独立 ctx（带 5s 超时），避免 ctx 被取消时 error 事件丢失导致 SSE 消费端无限等待。
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

	finalCtx, finalCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finalCancel()

	_, _ = w.pub.PublishPayload(finalCtx, eventbus.EventError, run.ID, run.ConversationThreadID, run.RequestID,
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
				"status":    "human_approval_required",
				"approval":  approval,
				"run_id":    run.ID,
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

// publishQuestion 发布 ask_user 提问事件。
// 前端 SSE 流收到 status=ask_user_question_required 后弹窗展示问题列表，
// 用户填写答案后提交 resume 请求。
func (w *Worker) publishQuestion(ctx context.Context, run *entity.AgentRun, ie *InterruptedError) {
	// 1. 先发 message 事件：items 中包含 questions，触发前端提问弹窗。
	msgPayload := map[string]any{
		"items": []map[string]any{
			{
				"status":        "ask_user_question_required",
				"questions":     ie.Questions,
				"run_id":        run.ID,
				"thread_id":     run.ConversationThreadID,
				"checkpoint_id": ie.CheckpointID,
			},
		},
	}
	_, _ = w.pub.PublishPayload(ctx, eventbus.EventMessage, run.ID, run.ConversationThreadID, run.RequestID, msgPayload)

	// 2. 再发 end 事件标记中断终态。
	_, _ = w.pub.PublishPayload(ctx, eventbus.EventEnd, run.ID, run.ConversationThreadID, run.RequestID,
		eventbus.EndPayload{
			Status: eventbus.StatusInterrupted,
		})
}

// saveInterruptInfo 保存中断信息到 run 记录（checkpointID + approval/question info），
// 供 resume 时读取。
func (w *Worker) saveInterruptInfo(_ context.Context, run *entity.AgentRun, ie *InterruptedError) {
	run.CheckpointID = ie.CheckpointID

	if ie.IsQuestion {
		// ask_user 提问中断
		run.ApprovalInfo = entity.JSON{
			"type":          "ask_user",
			"questions":     ie.Questions,
			"checkpoint_id": ie.CheckpointID,
			"interrupt_ids": ie.InterruptIDs,
		}
	} else {
		// 工具审批中断
		run.ApprovalInfo = entity.JSON{
			"action_requests": ie.Requests,
			"checkpoint_id":   ie.CheckpointID,
			"interrupt_ids":   ie.InterruptIDs,
		}
	}
	if err := w.runRepo.Update(run); err != nil {
		w.logger.Warn("worker 保存中断信息失败", zap.String("run_id", run.ID), zap.Error(err))
	}
}

// extractReasoningFromSchemaMsg 从 schema.Message 中提取推理内容。
// 优先读 ReasoninContent 字段（eino v0.10.0+），兜底从 AssistantGenMultiContent 的 reasoning part 提取。
func extractReasoningFromSchemaMsg(m *schema.Message) string {
	if m == nil {
		return ""
	}
	if m.ReasoningContent != "" {
		return m.ReasoningContent
	}
	for _, part := range m.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeReasoning && part.Reasoning != nil {
			return part.Reasoning.Text
		}
	}
	return ""
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
