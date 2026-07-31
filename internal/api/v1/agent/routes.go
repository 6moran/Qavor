package agent

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册智能体路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	agentGroup := r.Group("/agent")
	agentGroup.Use(middleware.Auth())
	{
		agentGroup.POST("", ctrl.Create)
		agentGroup.GET("/list", ctrl.List)
		agentGroup.GET("/default", ctrl.GetDefault)
		agentGroup.GET("/:slug", ctrl.Get)
		agentGroup.PUT("/:slug", ctrl.Update)
		agentGroup.DELETE("/:slug", ctrl.Delete)
		agentGroup.POST("/:slug/default", ctrl.SetDefault)
	}
}
