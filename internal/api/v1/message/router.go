package message

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册消息路由
// 注意：路由组用 /:id 表示 conversation_id，子路由用 /:msg_id 表示 message_id
// Gin 路由树不允许同一位置出现不同参数名的通配符
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	messages := router.Group("/conversations/:id/messages")
	{
		messages.POST("", ctrl.CreateMessage)
		messages.GET("", ctrl.ListMessages)
		messages.GET("/latest", ctrl.GetLatestMessage)
		messages.GET("/:msg_id", ctrl.GetMessage)
		messages.PUT("/:msg_id", ctrl.UpdateMessage)
		messages.DELETE("/:msg_id", ctrl.DeleteMessage)
	}
}
