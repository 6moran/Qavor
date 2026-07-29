// Package knowledge_file 提供知识库文件上传和文件 CRUD 的 HTTP API。
package knowledge_file

import (
	"Qavor/internal/middleware"
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 负责文件请求参数绑定，并将存储和持久化操作交给文件业务层。
type Controller struct{ service service.KnowledgeFileService }

// NewController 创建知识库文件 API 控制器。
func NewController(service service.KnowledgeFileService) *Controller {
	return &Controller{service: service}
}

// Upload 接收 multipart 的 file 字段；kb_id 为可选参数
func (ctrl *Controller) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少上传文件")
		return
	}
	result, err := ctrl.service.Upload(c.Query("kb_id"), middleware.GetUsername(c), file)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// List 获取指定知识库下的文件列表
func (ctrl *Controller) List(c *gin.Context) {
	var req request.KnowledgeFileListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.List(c.Param("kb_id"), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// Get 根据 kb_id 和 doc_id 获取文件详情，避免跨知识库查询
func (ctrl *Controller) Get(c *gin.Context) {
	result, err := ctrl.service.Get(c.Param("kb_id"), c.Param("doc_id"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// Delete 删除数据库文件记录，并由业务层同步清理对象存储
func (ctrl *Controller) Delete(c *gin.Context) {
	if err := ctrl.service.Delete(c.Param("kb_id"), c.Param("doc_id")); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}
