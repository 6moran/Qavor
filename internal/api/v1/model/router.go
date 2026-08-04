package model

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册模型路由
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	models := router.Group("/models")
	{
		models.POST("", ctrl.CreateModel)
		models.GET("", ctrl.ListModels)

		// 供应商相关路由
		models.GET("/providers", ctrl.GetProviders)
		models.GET("/providers/:name", ctrl.GetProviderByName)

		// 模型详情路由
		// 静态认证路由必须注册在 /:id 路由之前，避免 Gin 把 "test" 当作模型 ID。
		models.POST("/test", middleware.Auth(), ctrl.TestConnection)
		models.GET("/:id", ctrl.GetModel)
		models.PUT("/:id", ctrl.UpdateModel)
		models.DELETE("/:id", ctrl.DeleteModel)
	}
}
