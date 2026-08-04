package model

import "github.com/gin-gonic/gin"

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
		models.GET("/:id", ctrl.GetModel)
		models.PUT("/:id", ctrl.UpdateModel)
		models.DELETE("/:id", ctrl.DeleteModel)
	}
}
