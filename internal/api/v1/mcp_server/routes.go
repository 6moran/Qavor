package mcp_server

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册MCP服务器路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	mcpGroup := r.Group("/mcp")
	mcpGroup.Use(middleware.Auth())
	{
		mcpGroup.POST("", ctrl.Create)
		mcpGroup.GET("/list", ctrl.List)
		mcpGroup.GET("/:name", ctrl.Get)
		mcpGroup.PUT("/:name", ctrl.Update)
		mcpGroup.DELETE("/:name", ctrl.Delete)
		mcpGroup.POST("/:name/enable", ctrl.Enable)
		mcpGroup.POST("/:name/disable", ctrl.Disable)
	}
}
