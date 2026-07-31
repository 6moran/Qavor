package tool

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册工具路由
func (c *Controller) RegisterRoutes(r *gin.RouterGroup) {
	tools := r.Group("/system/tools")
	{
		tools.GET("", c.GetTools)
		tools.GET("/options", c.GetToolOptions)
	}
}
