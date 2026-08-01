package chat

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册聊天路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	chatGroup := r.Group("/chat")
	chatGroup.Use(middleware.Auth())
	{
		// 流式对话（SSE）
		chatGroup.POST("/stream", ctrl.sseCtrl.Stream)

		// 取消生成
		chatGroup.POST("/cancel", ctrl.sseCtrl.Cancel)

		// 文件上传
		chatGroup.POST("/upload", ctrl.sseCtrl.UploadFile)

		// 文件处理
		chatGroup.POST("/process-file", ctrl.sseCtrl.ProcessFile)
	}
}
