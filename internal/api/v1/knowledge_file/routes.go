package knowledge_file

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册需要 JWT 认证的知识库文件路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/knowledge")
	group.Use(middleware.Auth())
	group.POST("/files/upload", ctrl.Upload)
	group.POST("/databases/:kb_id/folders", ctrl.CreateFolder)
	group.GET("/databases/:kb_id/documents", ctrl.List)
	group.GET("/databases/:kb_id/documents/search", ctrl.Search)
	group.DELETE("/databases/:kb_id/documents/batch", ctrl.BatchDelete)
	group.GET("/databases/:kb_id/documents/:doc_id", ctrl.Get)
	group.GET("/databases/:kb_id/documents/:doc_id/content", ctrl.Preview)
	group.GET("/databases/:kb_id/documents/:doc_id/download", ctrl.Download)
	group.DELETE("/databases/:kb_id/documents/:doc_id", ctrl.Delete)
}
