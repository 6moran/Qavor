package dashboard

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册仪表盘路由
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/dashboard")
	group.Use(middleware.Auth())
	{
		group.GET("/stats/calls/timeseries", ctrl.GetCallTimeseries)
	}
}
