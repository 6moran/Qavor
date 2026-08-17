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
		group.GET("/:trace_id/spans/:span_id", ctrl.GetSpan)
	}

	// Run 反向定位：通过 run_id 查询关联的 trace_id
	runsGroup := r.Group("/runs")
	runsGroup.Use(middleware.Auth())
	{
		runsGroup.GET("/:run_id/trace", ctrl.GetTraceByRunID)
	}
}
