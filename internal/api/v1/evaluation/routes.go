package evaluation

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 RAG 评估路由（数据集管理 + 评估运行）。
// 与前端 evaluationApi 契约一致，路径统一使用 /api/v1 前缀（由外部 v1 分组提供）。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/evaluation")
	group.Use(middleware.Auth())

	// 数据集（基准）管理
	group.POST("/databases/:kb_id/datasets/upload", ctrl.UploadDataset)
	group.GET("/databases/:kb_id/datasets", ctrl.ListDatasets)
	group.GET("/databases/:kb_id/datasets/:dataset_id", ctrl.GetDataset)
	group.POST("/databases/:kb_id/datasets/generate", ctrl.GenerateDataset)
	group.POST("/databases/:kb_id/datasets/:dataset_id/resume", ctrl.ResumeDatasetGeneration)
	group.DELETE("/datasets/:dataset_id", ctrl.DeleteDataset)
	group.GET("/datasets/:dataset_id/download", ctrl.DownloadDataset)

	// 评估运行
	group.POST("/databases/:kb_id/runs", ctrl.RunEvaluation)
	group.GET("/databases/:kb_id/runs", ctrl.ListRuns)
	group.GET("/databases/:kb_id/runs/:run_id", ctrl.GetRunResults)
	group.DELETE("/databases/:kb_id/runs/:run_id", ctrl.DeleteRun)
}
