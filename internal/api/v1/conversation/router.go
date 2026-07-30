package conversation

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册会话路由
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	conversations := router.Group("/conversations")
	{
		conversations.POST("", ctrl.CreateConversation)
		conversations.GET("", ctrl.ListConversations)
		conversations.GET("/:id", ctrl.GetConversation)
		conversations.PUT("/:id", ctrl.UpdateConversation)
		conversations.DELETE("/:id", ctrl.DeleteConversation)
		conversations.PUT("/:id/close", ctrl.CloseConversation)
		conversations.PUT("/:id/archive", ctrl.ArchiveConversation)
	}
}
