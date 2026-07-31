package processing_job

import (
	"Qavor/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/knowledge/processing-jobs")
	group.Use(middleware.Auth())
	group.GET("", ctrl.List)
	group.GET("/:job_id", ctrl.Get)
	group.POST("/:job_id/retry", ctrl.Retry)
}
