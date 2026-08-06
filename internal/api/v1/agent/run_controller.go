package agent

import (
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
	runRepo repository.AgentRunRepository
	queue   *run.RequestQueue
	worker  *run.Worker
	logger  *zap.Logger
}

// NewRunController 创建 Run 控制器
func NewRunController(runRepo repository.AgentRunRepository, queue *run.RequestQueue, worker *run.Worker, logger *zap.Logger) *RunController {
	return &RunController{runRepo: runRepo, queue: queue, worker: worker, logger: logger}
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
