package message

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册消息路由
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	messages := router.Group("/conversations/:conversation_id/messages")
	{
		messages.POST("", ctrl.CreateMessage)
		messages.GET("", ctrl.ListMessages)
		messages.GET("/latest", ctrl.GetLatestMessage)
		messages.GET("/:id", ctrl.GetMessage)
		messages.PUT("/:id", ctrl.UpdateMessage)
		messages.DELETE("/:id", ctrl.DeleteMessage)
	}
}
