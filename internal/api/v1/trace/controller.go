package trace

import (
	"time"

	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 链路追踪控制器
type Controller struct {
	traceService service.TraceService
}

// NewController 创建控制器
func NewController(traceService service.TraceService) *Controller {
	return &Controller{traceService: traceService}
}

// parseTime 解析 RFC3339 时间，空串返回零值
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ListTraces 获取 Trace 列表
func (ctrl *Controller) ListTraces(c *gin.Context) {
	var req request.TraceListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	page, pageSize := req.Page, req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	items, total, err := ctrl.traceService.ListTraces(c.Request.Context(), service.TraceListFilter{
		Keyword:        req.Keyword,
		AgentSlug:      req.AgentSlug,
		ConversationID: req.ConversationID,
		Status:         req.Status,
		Model:          req.Model,
		Tool:           req.Tool,
		ErrorOnly:      req.ErrorOnly,
		MismatchOnly:   req.MismatchOnly,
		From:           parseTime(req.From),
		To:             parseTime(req.To),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		logger.Error("获取 Trace 列表失败", zap.Error(err))
		response.InternalServerError(c)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total})
}

// GetTrace 获取 Trace 详情（头部 + spans 平铺 + diagnostics）
func (ctrl *Controller) GetTrace(c *gin.Context) {
	traceID := c.Param("trace_id")
	if traceID == "" {
		response.BadRequest(c, "trace_id 不能为空")
		return
	}
	detail, err := ctrl.traceService.GetTraceDetail(c.Request.Context(), traceID)
	if err != nil {
		if errors.IsBizError(err) {
			response.BizError(c, err)
		} else {
			logger.Error("获取 Trace 详情失败", zap.String("trace_id", traceID), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, detail)
}

// GetTraceByRunID 通过 run_id 反查 trace_id
// GET /api/v1/runs/:run_id/trace
func (ctrl *Controller) GetTraceByRunID(c *gin.Context) {
	runID := c.Param("run_id")
	if runID == "" {
		response.BadRequest(c, "run_id 不能为空")
		return
	}
	traceID, err := ctrl.traceService.GetTraceByRunID(c.Request.Context(), runID)
	if err != nil {
		if errors.IsBizError(err) {
			response.BizError(c, err)
		} else {
			logger.Error("通过 run_id 反查 trace 失败", zap.String("run_id", runID), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, gin.H{"trace_id": traceID})
}
