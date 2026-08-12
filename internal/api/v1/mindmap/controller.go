// Package mindmap 提供知识导图的 HTTP API。
package mindmap

import (
	"Qavor/internal/service"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 负责知识导图请求绑定和统一响应转换。
type Controller struct {
	service service.MindmapService
}

// NewController 创建知识导图控制器。
func NewController(svc service.MindmapService) *Controller {
	return &Controller{service: svc}
}

// ListDatabases 获取可生成知识导图的知识库列表。
func (ctrl *Controller) ListDatabases(c *gin.Context) {
	result, err := ctrl.service.ListDatabases(c.Request.Context())
	if err != nil {
		response.InternalServerErrorWithDetail(c, err)
		return
	}
	response.Success(c, result)
}

// ListFiles 获取知识库中可参与导图生成的文件。
func (ctrl *Controller) ListFiles(c *gin.Context) {
	result, err := ctrl.service.ListFiles(c.Request.Context(), c.Param("kb_id"))
	if err != nil {
		response.InternalServerErrorWithDetail(c, err)
		return
	}
	response.Success(c, result)
}

// Get 获取知识库已保存的知识导图。
func (ctrl *Controller) Get(c *gin.Context) {
	result, err := ctrl.service.Get(c.Request.Context(), c.Param("kb_id"))
	if err != nil {
		response.InternalServerErrorWithDetail(c, err)
		return
	}
	response.Success(c, result)
}

// GetDiff 获取导图来源文件的增量差异。
func (ctrl *Controller) GetDiff(c *gin.Context) {
	result, err := ctrl.service.GetDiff(c.Request.Context(), c.Param("kb_id"))
	if err != nil {
		response.InternalServerErrorWithDetail(c, err)
		return
	}
	response.Success(c, result)
}

// Generate 生成或增量更新知识导图。
func (ctrl *Controller) Generate(c *gin.Context) {
	var req service.GenerateMindmapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.Generate(c.Request.Context(), c.Param("kb_id"), &req)
	if err != nil {
		response.InternalServerErrorWithDetail(c, err)
		return
	}
	response.Success(c, result)
}
