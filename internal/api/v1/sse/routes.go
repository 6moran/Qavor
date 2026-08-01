package sse

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 SSE 相关路由
func RegisterRoutes(r *gin.RouterGroup, ctrl *Controller) {
	sseGroup := r.Group("/chat")
	sseGroup.Use(middleware.Auth())
	{
		// 流式对话（SSE）
		sseGroup.POST("/stream", ctrl.Stream)

		// 取消生成
		sseGroup.POST("/cancel", ctrl.Cancel)

		// 文件上传
		sseGroup.POST("/upload", ctrl.UploadFile)

		// 文件处理
		sseGroup.POST("/process-file", ctrl.ProcessFile)
	}
}
