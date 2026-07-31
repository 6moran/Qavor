// Package knowledge_file 提供知识库文件上传和文件 CRUD 的 HTTP API。
package knowledge_file

import (
	"net/url"
	"strings"

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

// CreateFolder 创建知识库中的元数据文件夹。
func (ctrl *Controller) CreateFolder(c *gin.Context) {
	var req request.CreateKnowledgeFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.CreateFolder(c.Param("kb_id"), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	c.JSON(202, response.Response{Code: 0, Message: "文件已进入解析队列", Data: result})
}

// Upload 接收 multipart 的 file 字段；kb_id 为可选参数
func (ctrl *Controller) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少上传文件")
		return
	}
	autoIndex := strings.EqualFold(c.Query("auto_index"), "true")
	result, err := ctrl.service.Upload(c.Query("kb_id"), c.Query("parent_id"), autoIndex, file)
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

// Search 按文件名、原始文件名或存储路径搜索当前知识库的文件。
func (ctrl *Controller) Search(c *gin.Context) {
	var req request.SearchKnowledgeFileRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.Search(c.Param("kb_id"), req.Query, req.Offset, req.Limit)
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

// Preview 返回解析后的 Markdown 或原始文件的有限文本内容。
func (ctrl *Controller) Preview(c *gin.Context) {
	result, err := ctrl.service.Preview(c.Param("kb_id"), c.Param("doc_id"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// Download 以附件形式流式返回原始文件。
func (ctrl *Controller) Download(c *gin.Context) {
	result, err := ctrl.service.Download(c.Param("kb_id"), c.Param("doc_id"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	defer result.Reader.Close()
	filename := result.Filename
	if filename == "" {
		filename = "download"
	}
	contentType := result.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(strings.ReplaceAll(filename, "\\", "_")))
	c.DataFromReader(200, result.Size, contentType, result.Reader, nil)
}

// BatchDelete 批量删除指定知识库中的文件。
func (ctrl *Controller) BatchDelete(c *gin.Context) {
	var req request.BatchDeleteKnowledgeFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.BatchDelete(c.Param("kb_id"), req.FileIDs)
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
