package knowledge_base

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 知识库 CRUD 与检索测试路由
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/knowledge")
	group.Use(middleware.Auth())
	group.GET("/chunk-presets", ctrl.ChunkPresets)
	group.GET("/databases", ctrl.List)
	group.POST("/databases", ctrl.Create)
	group.GET("/databases/:kb_id", ctrl.Get)
	group.PUT("/databases/:kb_id", ctrl.Update)
	group.DELETE("/databases/:kb_id", ctrl.Delete)
	// 检索测试路由
	group.POST("/databases/:kb_id/query-test", ctrl.QueryTest)
	group.GET("/databases/:kb_id/query-params", ctrl.GetQueryParams)
	group.PUT("/databases/:kb_id/query-params", ctrl.UpdateQueryParams)
	group.GET("/databases/:kb_id/sample-questions", ctrl.GetSampleQuestions)
	group.POST("/databases/:kb_id/sample-questions", ctrl.GenerateSampleQuestions)
	// AI 生成/润色知识库描述（新建与编辑表单共用）
	group.POST("/generate-description", ctrl.GenerateDescription)
}
