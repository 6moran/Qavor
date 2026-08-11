package conversation

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册会话路由
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	conversations := router.Group("/conversations")
	conversations.Use(middleware.Auth())
	{
		conversations.POST("", ctrl.CreateConversation)
		conversations.GET("", ctrl.ListConversations)
		conversations.GET("/:id", ctrl.GetConversation)
		conversations.PUT("/:id", ctrl.UpdateConversation)
		conversations.DELETE("/:id", ctrl.DeleteConversation)
		conversations.PUT("/:id/close", ctrl.CloseConversation)
		conversations.PUT("/:id/archive", ctrl.ArchiveConversation)
		conversations.POST("/:id/clear-context", ctrl.ClearContext)
	}
}
