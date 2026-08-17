package sse

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 SSE 相关路由
func RegisterRoutes(r *gin.RouterGroup, ctrl *Controller) {
	sseGroup := r.Group("/sse")
	sseGroup.Use(middleware.Auth())
	{
		// 建立 SSE 连接
		sseGroup.GET("/connect", ctrl.Connect)

		// 获取连接信息
		sseGroup.GET("/info", ctrl.GetConnectionInfo)
	}
}
