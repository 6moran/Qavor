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
		chatGroup.POST("", ctrl.Chat)
	}
}
