package agent

import (
	stdctx "context"
	"strconv"

	"Qavor/internal/context"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/run"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RunController Run 状态查询 / 取消 / 队列操作控制器
type RunController struct {
	runRepo          repository.AgentRunRepository
	queue            *run.RequestQueue
	worker           *run.Worker
	logger           *zap.Logger
	contextMgr       context.ContextManager
	conversationRepo repository.ConversationRepository
	todoStore        *run.TodoStore
}

// NewRunController 创建 Run 控制器
func NewRunController(
	runRepo repository.AgentRunRepository,
	queue *run.RequestQueue,
	worker *run.Worker,
	logger *zap.Logger,
	contextMgr context.ContextManager,
	conversationRepo repository.ConversationRepository,
	todoStore *run.TodoStore,
) *RunController {
	return &RunController{
		runRepo:          runRepo,
		queue:            queue,
		worker:           worker,
		logger:           logger,
		contextMgr:       contextMgr,
		conversationRepo: conversationRepo,
		todoStore:        todoStore,
	}
}

// toRunResponse 实体转响应
func toRunResponse(r *entity.AgentRun) gin.H {
	return gin.H{
		"id":                     r.ID,
		"conversation_thread_id": r.ConversationThreadID,
		"agent_slug":             r.AgentSlug,
		"status":                 r.Status,
		"request_id":             r.RequestID,
		"run_type":               r.RunType,
		"last_event_id":          r.LastEventID,
		"error_type":             r.ErrorType,
		"error_message":          r.ErrorMessage,
		"started_at":             r.StartedAt,
		"finished_at":            r.FinishedAt,
		"created_at":             r.CreatedAt,
	}
}

// GetRun GET /api/v1/agent/runs/:runId
func (ctrl *RunController) GetRun(c *gin.Context) {
	runID := c.Param("runId")
	r, err := ctrl.runRepo.GetByID(runID)
	if err != nil {
		logger.Error("获取 Run 失败", zap.String("run_id", runID), zap.Error(err))
		response.InternalServerError(c)
		return
	}
	if r == nil {
		response.NotFound(c, "run 不存在")
		return
	}
	response.Success(c, toRunResponse(r))
}

// CancelRun POST /api/v1/agent/runs/:runId/cancel
func (ctrl *RunController) CancelRun(c *gin.Context) {
	runID := c.Param("runId")
	r, err := ctrl.runRepo.GetByID(runID)
	if err != nil {
		response.InternalServerError(c)
		return
	}
	if r == nil {
		response.NotFound(c, "run 不存在")
		return
	}
	if r.IsTerminal() {
		response.BadRequest(c, "run 已终态，无法取消")
		return
	}

	// 若运行中，调用 worker 取消
	if ctrl.worker != nil {
		ctrl.worker.CancelRun(runID)
	}
	// 若仍在排队，从队列移除
	_, _ = ctrl.queue.Remove(c.Request.Context(), runID)
	// 更新状态为 cancelled（若 worker 尚未置终态）
	_ = ctrl.runRepo.UpdateStatus(runID, entity.StatusCancelled, r.LastEventID)

	response.Success(c, toRunResponseCancel(runID))
}

func toRunResponseCancel(runID string) gin.H {
	return gin.H{"run_id": runID, "status": entity.StatusCancelled}
}

// GetRequest GET /api/v1/agent/requests/:requestId
func (ctrl *RunController) GetRequest(c *gin.Context) {
	requestID := c.Param("requestId")
	r, err := ctrl.runRepo.GetByRequestID(requestID)
	if err != nil {
		response.InternalServerError(c)
		return
	}
	if r == nil {
		response.NotFound(c, "request 不存在")
		return
	}
	response.Success(c, toRunResponse(r))
}

// CancelRequest POST /api/v1/agent/requests/:requestId/cancel
func (ctrl *RunController) CancelRequest(c *gin.Context) {
	requestID := c.Param("requestId")
	r, err := ctrl.runRepo.GetByRequestID(requestID)
	if err != nil {
		response.InternalServerError(c)
		return
	}
	if r == nil {
		response.NotFound(c, "request 不存在")
		return
	}
	// 排队中则移除；运行中则取消
	if ctrl.worker != nil {
		ctrl.worker.CancelRun(r.ID)
	}
	_, _ = ctrl.queue.Remove(c.Request.Context(), r.ID)
	if !r.IsTerminal() {
		_ = ctrl.runRepo.UpdateStatus(r.ID, entity.StatusCancelled, r.LastEventID)
	}
	response.Success(c, gin.H{"request_id": requestID, "status": entity.StatusCancelled})
}

// SteerRequest POST /api/v1/agent/requests/:requestId/steer
func (ctrl *RunController) SteerRequest(c *gin.Context) {
	requestID := c.Param("requestId")
	r, err := ctrl.runRepo.GetByRequestID(requestID)
	if err != nil {
		response.InternalServerError(c)
		return
	}
	if r == nil {
		response.NotFound(c, "request 不存在")
		return
	}
	if err := ctrl.queue.Steer(c.Request.Context(), r.ID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"request_id": requestID, "steered": true})
}

// ListThreadRequests GET /api/v1/agent/thread/:threadId/requests
func (ctrl *RunController) ListThreadRequests(c *gin.Context) {
	threadID := c.Param("threadId")
	items, err := ctrl.queue.ListThread(c.Request.Context(), threadID)
	if err != nil {
		logger.Error("列出线程排队请求失败", zap.String("thread_id", threadID), zap.Error(err))
		response.InternalServerError(c)
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

// ContinueThreadQueue POST /api/v1/agent/thread/:threadId/requests/continue
func (ctrl *RunController) ContinueThreadQueue(c *gin.Context) {
	threadID := c.Param("threadId")
	if err := ctrl.queue.ResumeThread(c.Request.Context(), threadID); err != nil {
		response.Error(c, errors.CodeInternalError, err.Error())
		return
	}
	response.Success(c, gin.H{"thread_id": threadID, "resumed": true})
}

// GetAgentState GET /api/v1/agent/thread/:threadId/agent-state
// 返回 Agent 状态面板数据（token 用量、待办、文件、子 Agent 运行）
// :threadId 同时支持数字 ID（如 137）和 UUID（如 7d936311-...），
// 前端全链路使用数字 ID，旧版 SSE/Run 相关调用可能使用 UUID。
func (ctrl *RunController) GetAgentState(c *gin.Context) {
	threadID := c.Param("threadId")

	// 优先按数字 ID 查询（前端 currentChatId 是数字 ID）
	var conv *entity.Conversation
	var err error
	if numericID, parseErr := strconv.ParseUint(threadID, 10, 64); parseErr == nil {
		conv, err = ctrl.conversationRepo.FindByID(uint(numericID))
	} else {
		// 非数字，按 UUID thread_id 查询
		conv, err = ctrl.conversationRepo.FindByThreadID(threadID)
	}
	if err != nil {
		logger.Error("GetAgentState: 查找会话失败", zap.String("thread_id", threadID), zap.Error(err))
		response.InternalServerError(c)
		return
	}
	if conv == nil {
		response.NotFound(c, "会话不存在")
		return
	}

	// 降级：若 contextMgr 未注入，返回空状态
	if ctrl.contextMgr == nil {
		response.Success(c, gin.H{"agent_state": gin.H{
			"token_usage":    nil,
			"todos":          []interface{}{},
			"files":          gin.H{},
			"subagent_runs":  []interface{}{},
			"artifacts":      []interface{}{},
			"memory_metrics": nil,
		}})
		return
	}

	// 获取 token_usage 等（contextMgr 聚合）
	state, err := ctrl.contextMgr.GetAgentState(c.Request.Context(), conv.ID)
	if err != nil {
		logger.Error("GetAgentState: 获取状态失败", zap.Uint("conv", conv.ID), zap.Error(err))
		response.InternalServerError(c)
		return
	}
	if state == nil {
		state = &context.AgentState{
			Todos:        []context.AgentTodo{},
			Files:        map[string]context.AgentFile{},
			SubagentRuns: []context.AgentSubagentRun{},
			Artifacts:    []string{},
		}
	}

	// 读取 Redis 持久化的 TODO 列表
	ctrl.enrichTodos(c.Request.Context(), conv.ID, state)

	// 聚合 subagent_runs：查询父会话下的子智能体线程
	ctrl.enrichSubagentRuns(conv.ID, state)

	response.Success(c, gin.H{"agent_state": state})
}

// enrichSubagentRuns 查询子智能体线程运行状态并填充到 AgentState
func (ctrl *RunController) enrichSubagentRuns(parentConvID uint, state *context.AgentState) {
	threads, err := ctrl.runRepo.ListSubagentThreadsByParent(parentConvID)
	if err != nil {
		ctrl.logger.Warn("查询子智能体线程失败", zap.Uint("parent_conv", parentConvID), zap.Error(err))
		return
	}

	for _, t := range threads {
		// 查询子线程最新的 run 以获取运行状态
		runStatus := "running"
		if runs, _, e := ctrl.runRepo.ListByThread(t.ChildThreadID, 0, 1); e == nil && len(runs) > 0 {
			runStatus = mapRunStatusToFrontend(runs[0].Status)
		}

		state.SubagentRuns = append(state.SubagentRuns, context.AgentSubagentRun{
			ID:            t.CreatedByRunID,
			RunID:         t.CreatedByRunID,
			ChildThreadID: t.ChildThreadID,
			SubagentSlug:  t.SubagentSlug,
			Status:        runStatus,
			Description:   "",
		})
	}
}

// mapRunStatusToFrontend 将后端 Run 状态映射为前端 subagent_runs 期望的状态
func mapRunStatusToFrontend(status string) string {
	switch status {
	case entity.StatusCompleted:
		return "completed"
	case entity.StatusFailed, entity.StatusCancelled:
		return "failed"
	default: // pending / running / interrupted
		return "running"
	}
}

// enrichTodos 从 Redis 读取持久化的 TODO 列表并填充到 AgentState
func (ctrl *RunController) enrichTodos(ctx stdctx.Context, conversationID uint, state *context.AgentState) {
	if ctrl.todoStore == nil || conversationID == 0 {
		return
	}
	todos, err := ctrl.todoStore.GetTodos(ctx, conversationID)
	if err != nil {
		ctrl.logger.Warn("读取 TODO 列表失败", zap.Uint("conv", conversationID), zap.Error(err))
		return
	}
	for _, t := range todos {
		state.Todos = append(state.Todos, context.AgentTodo{
			ID:      t.ID,
			Content: t.Content,
			Status:  t.Status,
		})
	}
}
