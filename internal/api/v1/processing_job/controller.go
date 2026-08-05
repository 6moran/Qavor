package processing_job

import (
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Controller struct{ service service.ProcessingJobService }

func NewController(service service.ProcessingJobService) *Controller {
	return &Controller{service: service}
}

func (ctrl *Controller) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	result, err := ctrl.service.List(limit)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取处理任务列表失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取处理任务列表失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, result)
}

func (ctrl *Controller) Get(c *gin.Context) {
	result, err := ctrl.service.Get(c.Param("job_id"))
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取处理任务失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取处理任务失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, result)
}

func (ctrl *Controller) Retry(c *gin.Context) {
	result, err := ctrl.service.Retry(c.Param("job_id"))
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，重试处理任务失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("重试处理任务失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	c.JSON(202, response.Response{Code: 0, Message: "解析任务已进入队列", Data: result})
}
