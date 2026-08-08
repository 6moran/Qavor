package trace

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册链路追踪路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/traces")
	group.Use(middleware.Auth())
	{
		group.GET("", ctrl.ListTraces)
		group.GET("/:trace_id", ctrl.GetTrace)
	}
}
