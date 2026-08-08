package knowledge_base

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 知识库 CRUD 路由
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/knowledge")
	group.Use(middleware.Auth())
	group.GET("/chunk-presets", ctrl.ChunkPresets)
	group.GET("/databases", ctrl.List)
	group.POST("/databases", ctrl.Create)
	group.GET("/databases/:kb_id", ctrl.Get)
	group.PUT("/databases/:kb_id", ctrl.Update)
	group.DELETE("/databases/:kb_id", ctrl.Delete)
}
