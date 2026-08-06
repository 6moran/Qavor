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
		// 普通聊天（非流式）
		chatGroup.POST("", ctrl.Chat)
		chatGroup.POST("/call", ctrl.Chat)

		// 流式聊天（SSE）
		chatGroup.POST("/stream", ctrl.ChatStream)
	}
}
