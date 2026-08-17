// Package evaluation 提供 RAG 评估基准管理与评估运行的 HTTP API。
package evaluation

import (
	"io"
	"strconv"

	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 负责评估请求参数绑定、业务服务调用和统一响应转换。
type Controller struct {
	service service.EvaluationService
}

// NewController 创建评估 API 控制器。
func NewController(service service.EvaluationService) *Controller {
	return &Controller{service: service}
}

// UploadDataset 上传 JSONL 评测数据集文件。
func (ctrl *Controller) UploadDataset(c *gin.Context) {
	kbID := c.Param("kb_id")
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少基准文件")
		return
	}
	name := c.PostForm("name")
	description := c.PostForm("description")

	file, err := fileHeader.Open()
	if err != nil {
		response.BadRequest(c, "读取基准文件失败")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		response.BadRequest(c, "读取基准文件失败")
		return
	}

	result, err := ctrl.service.UploadDataset(c.Request.Context(), kbID, name, description, content, fileHeader.Filename)
	if err != nil {
		ctrl.handleError(c, "上传评估基准失败", err)
		return
	}
	response.Success(c, result)
}

// ListDatasets 获取知识库的数据集列表。
func (ctrl *Controller) ListDatasets(c *gin.Context) {
	result, err := ctrl.service.ListDatasets(c.Param("kb_id"))
	if err != nil {
		ctrl.handleError(c, "获取评估基准列表失败", err)
		return
	}
	response.Success(c, result)
}

// GetDataset 获取数据集详情（分页查看问答条目）。
func (ctrl *Controller) GetDataset(c *gin.Context) {
	page, pageSize := parsePagination(c)
	result, err := ctrl.service.GetDataset(c.Request.Context(), c.Param("kb_id"), c.Param("dataset_id"), page, pageSize)
	if err != nil {
		ctrl.handleError(c, "获取评估基准详情失败", err)
		return
	}
	response.Success(c, result)
}

// DeleteDataset 删除数据集。
func (ctrl *Controller) DeleteDataset(c *gin.Context) {
	if err := ctrl.service.DeleteDataset(c.Param("dataset_id")); err != nil {
		ctrl.handleError(c, "删除评估基准失败", err)
		return
	}
	response.Success(c, nil)
}

// DownloadDataset 下载数据集文件。
func (ctrl *Controller) DownloadDataset(c *gin.Context) {
	filename, content, err := ctrl.service.DownloadDataset(c.Param("dataset_id"))
	if err != nil {
		ctrl.handleError(c, "下载评估基准失败", err)
		return
	}
	// RFC 5987 编码文件名，兼容中文（前端按 filename*=UTF-8'' 解析）
	encoded := urlPathEscape(filename)
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+encoded)
	c.Header("Content-Type", "application/octet-stream")
	c.Data(200, "application/octet-stream", content)
}

// GenerateDataset 提交 AI 自动生成评测数据集任务。
func (ctrl *Controller) GenerateDataset(c *gin.Context) {
	var req service.GenerateDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.GenerateDataset(c.Request.Context(), c.Param("kb_id"), &req)
	if err != nil {
		ctrl.handleError(c, "提交基准生成任务失败", err)
		return
	}
	response.Success(c, result)
}

// ResumeDatasetGeneration 恢复中断的数据集生成任务。
func (ctrl *Controller) ResumeDatasetGeneration(c *gin.Context) {
	result, err := ctrl.service.ResumeDatasetGeneration(c.Request.Context(), c.Param("kb_id"), c.Param("dataset_id"))
	if err != nil {
		ctrl.handleError(c, "恢复基准生成失败", err)
		return
	}
	response.Success(c, result)
}

// RunEvaluation 发起一次 RAG 评估运行。
func (ctrl *Controller) RunEvaluation(c *gin.Context) {
	var req service.RunEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.RunEvaluation(c.Request.Context(), c.Param("kb_id"), &req)
	if err != nil {
		ctrl.handleError(c, "启动评估失败", err)
		return
	}
	response.Success(c, result)
}

// ListRuns 获取评估运行列表。
func (ctrl *Controller) ListRuns(c *gin.Context) {
	result, err := ctrl.service.ListRuns(c.Param("kb_id"))
	if err != nil {
		ctrl.handleError(c, "获取评估运行列表失败", err)
		return
	}
	response.Success(c, result)
}

// GetRunResults 获取单次评估运行的结果。
func (ctrl *Controller) GetRunResults(c *gin.Context) {
	page, pageSize := parsePagination(c)
	errorOnly := c.DefaultQuery("error_only", "false") == "true"
	result, err := ctrl.service.GetRunResults(c.Request.Context(), c.Param("kb_id"), c.Param("run_id"), page, pageSize, errorOnly)
	if err != nil {
		ctrl.handleError(c, "获取评估结果失败", err)
		return
	}
	response.Success(c, result)
}

// DeleteRun 删除评估运行。
func (ctrl *Controller) DeleteRun(c *gin.Context) {
	if err := ctrl.service.DeleteRun(c.Param("kb_id"), c.Param("run_id")); err != nil {
		ctrl.handleError(c, "删除评估运行失败", err)
		return
	}
	response.Success(c, nil)
}

// parsePagination 解析分页查询参数。
func parsePagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "50"))
	return page, pageSize
}

// urlPathEscape 按 RFC 3986 转义路径段（保持与 JS encodeURIComponent 语义兼容）。
func urlPathEscape(s string) string {
	const hex = "0123456789ABCDEF"
	var b []byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.' || ch == '~' {
			b = append(b, ch)
		} else {
			b = append(b, '%', hex[ch>>4], hex[ch&0x0f])
		}
	}
	return string(b)
}

// handleError 统一错误响应：业务错误透传，其余按内部错误处理。
func (ctrl *Controller) handleError(c *gin.Context, stage string, err error) {
	if errors.IsBizError(err) {
		logger.Warn(stage, zap.Error(err))
		response.BizError(c, err)
		return
	}
	logger.Error(stage, zap.Error(err))
	response.InternalServerError(c)
}
