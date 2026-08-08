package workspace

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册需要 JWT 认证的工作区路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/workspace")
	group.Use(middleware.Auth())
	group.GET("/tree", ctrl.ListTree)
	group.GET("/file", ctrl.ReadFile)
	group.PUT("/file", ctrl.SaveFile)
	group.DELETE("/file", ctrl.DeleteFile)
	group.POST("/directory", ctrl.CreateDirectory)
	group.POST("/upload", ctrl.Upload)
	group.GET("/download", ctrl.Download)
}
