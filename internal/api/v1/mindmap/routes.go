package mindmap

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册知识导图 v1 路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/knowledge", middleware.Auth())
	group.GET("/mindmap/databases", ctrl.ListDatabases)
	group.GET("/databases/:kb_id/mindmap/files", ctrl.ListFiles)
	group.GET("/databases/:kb_id/mindmap", ctrl.Get)
	group.GET("/databases/:kb_id/mindmap/diff", ctrl.GetDiff)
	group.POST("/databases/:kb_id/mindmap/generate", ctrl.Generate)
}
