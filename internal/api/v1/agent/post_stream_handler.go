package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"Qavor/internal/eventbus"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/run"
	"Qavor/pkg/errors"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PostStreamHandler POST /api/v1/agent/runs 处理器
// 承担「新建 Run + 流式推送」与「resume 重连续传」两种语义
type PostStreamHandler struct {
	sub             *eventbus.Subscriber
	runRepo         repository.AgentRunRepository
	queue           *run.RequestQueue
	heartbeatPeriod time.Duration
	logger          *zap.Logger
}

// NewPostStreamHandler 创建 POST 流式处理器
func NewPostStreamHandler(sub *eventbus.Subscriber, runRepo repository.AgentRunRepository,
	queue *run.RequestQueue, heartbeatPeriod time.Duration, logger *zap.Logger) *PostStreamHandler {
	if heartbeatPeriod <= 0 {
		heartbeatPeriod = 15 * time.Second
	}
	return &PostStreamHandler{
		sub:             sub,
		runRepo:         runRepo,
		queue:           queue,
		heartbeatPeriod: heartbeatPeriod,
		logger:          logger,
	}
}

// CreateRunRequest 创建 Run / 断线重连请求体
type CreateRunRequest struct {
	Query            string          `json:"query"`
	AgentSlug        string          `json:"agent_slug"`
	ThreadID         string          `json:"thread_id" binding:"required"`
	Meta             json.RawMessage `json:"meta,omitempty"`
	ImageContent     string          `json:"image_content,omitempty"`
	ModelSpec        json.RawMessage `json:"model_spec,omitempty"`
	ToolApprovalMode string          `json:"tool_approval_mode,omitempty"`
	Resume           *ResumeParam    `json:"resume,omitempty"`
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

	var (
		runRec   *entity.AgentRun
		afterSeq string
	)

	if req.Resume != nil {
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

	now := time.Now()
	r := &entity.AgentRun{
		ID:                   runID,
		ConversationThreadID: req.ThreadID,
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

	// 入队
	item := run.QueueItem{
		RunID:     runID,
		ThreadID:  req.ThreadID,
		AgentSlug: req.AgentSlug,
		RequestID: requestID,
		Query:     req.Query,
		CreatedAt: now,
	}
	if err := h.queue.Enqueue(ctx, item); err != nil {
		_ = h.runRepo.UpdateStatus(runID, entity.StatusFailed, "")
		return nil, fmt.Errorf("入队失败: %w", err)
	}
	return r, nil
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
