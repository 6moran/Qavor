package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"Qavor/internal/eventbus"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/run"
	"Qavor/internal/trace"
	"Qavor/pkg/errors"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PostStreamHandler POST /api/v1/agent/runs 处理器
// 承担「新建 Run + 流式推送」与「resume 重连续传」两种语义
type PostStreamHandler struct {
	sub              *eventbus.Subscriber
	runRepo          repository.AgentRunRepository
	conversationRepo repository.ConversationRepository
	queue            *run.RequestQueue
	heartbeatPeriod  time.Duration
	logger           *zap.Logger
	tracer           *trace.Tracer
	traceLookup      RunTraceLookup
}

// RunTraceLookup 查询业务 Run 对应的 agent.run Span，用于恢复/重试延续原 Trace。
type RunTraceLookup interface {
	GetAgentRunSpan(ctx context.Context, runID string) (*trace.RunSpanRef, error)
}

// NewPostStreamHandler 创建 POST 流式处理器。
func NewPostStreamHandler(sub *eventbus.Subscriber, runRepo repository.AgentRunRepository,
	conversationRepo repository.ConversationRepository, queue *run.RequestQueue,
	heartbeatPeriod time.Duration, logger *zap.Logger,
	tracer *trace.Tracer, traceLookup RunTraceLookup) *PostStreamHandler {
	if heartbeatPeriod <= 0 {
		heartbeatPeriod = 15 * time.Second
	}
	return &PostStreamHandler{
		sub:              sub,
		runRepo:          runRepo,
		conversationRepo: conversationRepo,
		queue:            queue,
		heartbeatPeriod:  heartbeatPeriod,
		logger:           logger,
		tracer:           tracer,
		traceLookup:      traceLookup,
	}
}

// resumeTraceContext 将恢复任务接回原 agent.run 所在 Trace。
// 找不到原 Span 时保留当前 Context，并从 attempt=1 开始。
func resumeTraceContext(ctx context.Context, requestID string, ref *trace.RunSpanRef) (context.Context, int, string) {
	if ref == nil || ref.TraceID == "" || ref.SpanID == "" {
		return ctx, 1, ""
	}
	attempt := ref.Attempt + 1
	if attempt < 2 {
		attempt = 2
	}
	return trace.WithSpanContext(ctx, trace.SpanContext{
		TraceID:   ref.TraceID,
		SpanID:    ref.SpanID,
		RequestID: requestID,
		Sampled:   true,
	}), attempt, ref.SpanID
}

// CreateRunRequest 创建 Run / 断线重连 / 审批恢复 请求体
type CreateRunRequest struct {
	Query            string          `json:"query"`
	AgentSlug        string          `json:"agent_slug"`
	ThreadID         string          `json:"thread_id" binding:"required"`
	Meta             json.RawMessage `json:"meta,omitempty"`
	ImageContent     string          `json:"image_content,omitempty"`
	ModelSpec        json.RawMessage `json:"model_spec,omitempty"`
	ToolApprovalMode string          `json:"tool_approval_mode,omitempty"`
	Resume           *ResumeParam    `json:"resume,omitempty"`
	ApprovalDecision string          `json:"approval_decision,omitempty"` // 工具审批决策（"approve"/"reject"），created_by_run_id 非空时使用
	CreatedByRunID   string          `json:"created_by_run_id,omitempty"`
	QueuePolicy      string          `json:"queue_policy,omitempty"`
}

// ResumeParam 重连 / 工具审批恢复参数
type ResumeParam struct {
	RunID      string `json:"run_id"`
	LastSeq    string `json:"last_seq"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Decision   string `json:"decision,omitempty"`
}

// CreateRunAndStream POST /api/v1/agent/runs
func (h *PostStreamHandler) CreateRunAndStream(c *gin.Context) {
	var req CreateRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if h.tracer != nil {
		conversationID := uint(0)
		if parsed, parseErr := strconv.ParseUint(req.ThreadID, 10, 32); parseErr == nil {
			conversationID = uint(parsed)
		}
		entryType := "async"
		if req.CreatedByRunID != "" {
			entryType = "resume"
		} else if req.Resume != nil {
			entryType = "reconnect"
		}
		h.tracer.UpdateRequestMetadata(c.Request.Context(), conversationID, req.Query, entryType)
	}

	var (
		runRec   *entity.AgentRun
		afterSeq string
	)

	if req.CreatedByRunID != "" {
		// —— 审批恢复：从中断 Run 创建 resume Run ——
		newRun, err := h.approvalResume(c.Request.Context(), &req)
		if err != nil {
			response.Error(c, errors.CodeInternalError, err.Error())
			return
		}
		runRec = newRun
		afterSeq = "0-0"
	} else if req.Resume != nil {
		// —— 断线重连：校验已有 Run，不创建新 Run ——
		existing, err := h.runRepo.GetByID(req.Resume.RunID)
		if err != nil || existing == nil {
			response.NotFound(c, "run not found or expired")
			return
		}
		if existing.ConversationThreadID != req.ThreadID {
			response.BadRequest(c, "run 不属于该 thread")
			return
		}
		runRec = existing
		afterSeq = req.Resume.LastSeq
	} else {
		// —— 新建 Run：创建记录并入队 ——
		if req.Query == "" {
			response.BadRequest(c, "query 不能为空")
			return
		}
		if req.AgentSlug == "" {
			req.AgentSlug = "default"
		}
		if req.QueuePolicy == "" {
			req.QueuePolicy = run.QueuePolicyEnqueue
		}
		newRun, err := h.createAndEnqueue(c.Request.Context(), &req)
		if err != nil {
			response.Error(c, errors.CodeInternalError, err.Error())
			return
		}
		runRec = newRun
		afterSeq = "0-0"
	}

	// 写 SSE 响应头并 Flush，立即发一次心跳让前端确认连接
	h.setSSEHeaders(c)
	h.writeHeartbeat(c)

	// 启动订阅循环（含心跳 goroutine）
	h.subscribeLoop(c.Request.Context(), c, runRec, afterSeq)
}

// createAndEnqueue 创建 AgentRun 记录并入队
func (h *PostStreamHandler) createAndEnqueue(ctx context.Context, req *CreateRunRequest) (*entity.AgentRun, error) {
	runID := uuid.New().String()
	requestID := uuid.New().String()

	inputPayload := entity.JSON{}
	if req.Meta != nil {
		_ = json.Unmarshal(req.Meta, &inputPayload)
	}

	// 解析 conversation_id（用于关联 Run 和 Conversation）
	var conversationID *uint
	if conv, err := h.conversationRepo.FindByThreadID(ctx, req.ThreadID); err == nil && conv != nil {
		conversationID = &conv.ID
	} else if numID, parseErr := strconv.ParseUint(req.ThreadID, 10, 32); parseErr == nil {
		if conv, err := h.conversationRepo.FindByID(ctx, uint(numID)); err == nil && conv != nil {
			conversationID = &conv.ID
		}
	}

	now := time.Now()
	r := &entity.AgentRun{
		ID:                   runID,
		ConversationThreadID: req.ThreadID,
		ConversationID:       conversationID,
		AgentSlug:            req.AgentSlug,
		Status:               entity.StatusPending,
		RequestID:            requestID,
		CreatedByRunID:       req.CreatedByRunID,
		RunType:              "chat",
		InputPayload:         inputPayload,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := h.runRepo.Create(r); err != nil {
		return nil, fmt.Errorf("创建 Run 失败: %w", err)
	}

	// 入队：创建 queue.produce Span，QueueItem.Trace 提取该 Span 的 Context
	item := run.QueueItem{
		RunID:     runID,
		ThreadID:  req.ThreadID,
		AgentSlug: req.AgentSlug,
		RequestID: requestID,
		Query:     req.Query,
		CreatedAt: now,
	}
	// 旧字段 TraceID 仍保留兼容（从 ctx 读取）
	item.TraceID = trace.TraceIDFromContext(ctx)

	if h.tracer != nil {
		queueCtx, queueSpan := h.tracer.StartSpan(ctx, trace.SpanSpec{
			Kind:      "queue",
			Operation: "queue.produce",
			RunID:     runID,
			RequestID: requestID,
			Attributes: entity.JSON{
				"queue": "qavor:run:queue",
			},
		})
		if carrier, ok := trace.CarrierFromContext(queueCtx); ok {
			item.Trace = carrier
		}
		if err := h.queue.Enqueue(queueCtx, item); err != nil {
			queueSpan.End(trace.SpanEnd{
				Status:       trace.SpanStatusError,
				ErrorType:    "queue_enqueue",
				ErrorMessage: err.Error(),
			})
			_ = h.runRepo.UpdateStatus(ctx, runID, entity.StatusFailed, "")
			return nil, fmt.Errorf("入队失败: %w", err)
		}
		queueSpan.End(trace.SpanEnd{Status: trace.SpanStatusOK})
	} else {
		// tracer 未装配（迁移期）：直接入队
		if err := h.queue.Enqueue(ctx, item); err != nil {
			_ = h.runRepo.UpdateStatus(ctx, runID, entity.StatusFailed, "")
			return nil, fmt.Errorf("入队失败: %w", err)
		}
	}
	return r, nil
}

// approvalResume 处理工具审批恢复：从中断 Run 创建 resume Run。
// 前端提交 {resume: "approve"/"reject", created_by_run_id: "<中断run_id>"}，
// 从父 Run 记录取 checkpointID + approval info，构建 resume targets 后入队。
func (h *PostStreamHandler) approvalResume(ctx context.Context, req *CreateRunRequest) (*entity.AgentRun, error) {
	// 1. 加载中断 Run 记录
	parent, err := h.runRepo.GetByID(req.CreatedByRunID)
	if err != nil || parent == nil {
		return nil, fmt.Errorf("中断 Run 不存在: %s", req.CreatedByRunID)
	}

	// 2. 提取 checkpointID（worker 存在 ApprovalInfo 里）
	checkpointID := ""
	if parent.ApprovalInfo != nil {
		if cid, ok := parent.ApprovalInfo["checkpoint_id"].(string); ok {
			checkpointID = cid
		}
	}
	if checkpointID == "" {
		return nil, fmt.Errorf("中断 Run 缺少 checkpoint_id: %s", req.CreatedByRunID)
	}

	// 3. 构建 resume targets（中断地址 → 审批/回答决定）
	//    中断地址来自 ApprovalMiddleware 的 InterruptCtx.ID（中断 UUID），
	//    存在 ApprovalInfo.interrupt_ids 中。使用 UUID 作为 target key，
	//    与 eino 的 id2Addr → interrupt UUID → id2ResumeData 查找链匹配。
	//    降级：若 interrupt_ids 不存在，使用旧的中间件名 "qavor_approval" 保持兼容。
	decision := req.ApprovalDecision
	var targets map[string]any

	if parent.ApprovalInfo != nil {
		if idsRaw, ok := parent.ApprovalInfo["interrupt_ids"]; ok {
			if ids, ok := idsRaw.([]any); ok && len(ids) > 0 {
				targets = make(map[string]any, len(ids))
				for _, id := range ids {
					if s, ok := id.(string); ok {
						targets[s] = decision
					}
				}
			}
		}
	}

	h.logger.Info("approvalResume: targets after build", zap.Any("targets", targets), zap.String("decision", decision))

	// 4. 创建 resume Run
	runID := uuid.New().String()
	requestID := uuid.New().String()
	now := time.Now()
	resumeCtx := ctx
	attempt := 1
	resumeFromSpanID := ""
	if h.traceLookup != nil {
		ref, lookupErr := h.traceLookup.GetAgentRunSpan(ctx, parent.ID)
		if lookupErr != nil {
			return nil, fmt.Errorf("查询父 Run 链路失败: %w", lookupErr)
		}
		resumeCtx, attempt, resumeFromSpanID = resumeTraceContext(ctx, requestID, ref)
	}
	r := &entity.AgentRun{
		ID:                   runID,
		ConversationThreadID: req.ThreadID,
		AgentSlug:            parent.AgentSlug,
		Status:               entity.StatusPending,
		RequestID:            requestID,
		CreatedByRunID:       req.CreatedByRunID,
		RunType:              "resume",
		InputPayload:         entity.JSON{"decision": decision, "checkpoint_id": checkpointID},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := h.runRepo.Create(r); err != nil {
		return nil, fmt.Errorf("创建 resume Run 失败: %w", err)
	}

	// 5. 入队（携带 resume 参数）
	item := run.QueueItem{
		RunID:            runID,
		ThreadID:         req.ThreadID,
		AgentSlug:        parent.AgentSlug,
		RequestID:        requestID,
		Query:            "", // 恢复时 query 可空（由中断 Run 的输入决定）
		CreatedAt:        now,
		ApprovalMode:     req.ToolApprovalMode,
		ResumeRunID:      req.CreatedByRunID,
		CheckpointID:     checkpointID,
		Targets:          targets,
		Attempt:          attempt,
		ResumeFromRunID:  parent.ID,
		ResumeFromSpanID: resumeFromSpanID,
	}
	item.TraceID = trace.TraceIDFromContext(resumeCtx)
	if h.tracer != nil {
		queueCtx, queueSpan := h.tracer.StartSpan(resumeCtx, trace.SpanSpec{
			Kind:      "queue",
			Operation: "queue.produce",
			RunID:     runID,
			RequestID: requestID,
			Attributes: entity.JSON{
				"queue":               "qavor:run:queue",
				"resume":              true,
				"resume_from_run_id":  parent.ID,
				"resume_from_span_id": resumeFromSpanID,
				"attempt":             attempt,
			},
		})
		if carrier, ok := trace.CarrierFromContext(queueCtx); ok {
			item.Trace = carrier
		}
		if err := h.queue.Enqueue(queueCtx, item); err != nil {
			queueSpan.End(trace.SpanEnd{
				Status:       trace.SpanStatusError,
				ErrorType:    "queue_enqueue",
				ErrorMessage: err.Error(),
			})
			_ = h.runRepo.UpdateStatus(ctx, runID, entity.StatusFailed, "")
			return nil, fmt.Errorf("resume 入队失败: %w", err)
		}
		queueSpan.End(trace.SpanEnd{Status: trace.SpanStatusOK})
	} else {
		if err := h.queue.Enqueue(resumeCtx, item); err != nil {
			_ = h.runRepo.UpdateStatus(ctx, runID, entity.StatusFailed, "")
			return nil, fmt.Errorf("resume 入队失败: %w", err)
		}
	}
	return r, nil
}

// isAskUserResume 判断 approval_decision 是否为 ask_user 问答回复（JSON 对象）。
// ask_user 提交前的前端将 answers（map[string]string）序列化为 JSON 字符串；
// 工具审批的 decision 为 "approve"/"reject" 纯字符串。
func isAskUserResume(decision string) bool {
	return len(decision) > 0 && decision[0] == '{'
}

func (h *PostStreamHandler) setSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()
}

// writeHeartbeat 发送 SSE 注释行保活（前端自动忽略）
func (h *PostStreamHandler) writeHeartbeat(c *gin.Context) {
	_, _ = fmt.Fprint(c.Writer, ": heartbeat\n\n")
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
}

// subscribeLoop 主 goroutine 阻塞 XREAD；心跳 goroutine 周期写注释行
func (h *PostStreamHandler) subscribeLoop(ctx context.Context, c *gin.Context, run *entity.AgentRun, afterSeq string) {
	flusher, _ := c.Writer.(http.Flusher)

	// 写锁：心跳 goroutine 与事件写入互斥，避免帧字节交错
	var writeMu sync.Mutex
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()

	// 心跳 goroutine
	go func() {
		ticker := time.NewTicker(h.heartbeatPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				writeMu.Lock()
				_, _ = fmt.Fprint(c.Writer, ": heartbeat\n\n")
				if flusher != nil {
					flusher.Flush()
				}
				writeMu.Unlock()
			}
		}
	}()

	lastSeq := afterSeq
	for {
		select {
		case <-ctx.Done():
			return // 客户端断开
		default:
		}

		entries, err := h.sub.Read(ctx, run.ID, lastSeq)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			h.logger.Warn("SSE 读取事件失败", zap.String("run_id", run.ID), zap.Error(err))
			return
		}

		writeMu.Lock()
		for _, entry := range entries {
			terminal := h.writeSSEEvent(c, flusher, entry)
			lastSeq = entry.ID
			if terminal {
				writeMu.Unlock()
				return // 收到终态事件，关闭连接
			}
		}
		writeMu.Unlock()
	}
}

// writeSSEEvent 写入一个 SSE 事件，返回 true 表示终态事件（end/error）
func (h *PostStreamHandler) writeSSEEvent(c *gin.Context, flusher http.Flusher, entry eventbus.StreamEntry) bool {
	if entry.Event.EventType == "" {
		return false // 损坏的条目，跳过
	}
	envelope := eventbus.NewEnvelope(entry.Event)
	data, _ := json.Marshal(envelope)
	_, _ = fmt.Fprintf(c.Writer, "id: %s\nevent: %s\ndata: %s\n\n",
		entry.ID, entry.Event.EventType, string(data))
	if flusher != nil {
		flusher.Flush()
	}
	return entry.Event.EventType == eventbus.EventEnd || entry.Event.EventType == eventbus.EventError
}
