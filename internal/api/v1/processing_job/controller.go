package processing_job

import (
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Controller struct{ service service.ProcessingJobService }

func NewController(service service.ProcessingJobService) *Controller {
	return &Controller{service: service}
}

func (ctrl *Controller) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	result, err := ctrl.service.List(limit)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

func (ctrl *Controller) Get(c *gin.Context) {
	result, err := ctrl.service.Get(c.Param("job_id"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

func (ctrl *Controller) Retry(c *gin.Context) {
	result, err := ctrl.service.Retry(c.Param("job_id"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	c.JSON(202, response.Response{Code: 0, Message: "解析任务已进入队列", Data: result})
}
