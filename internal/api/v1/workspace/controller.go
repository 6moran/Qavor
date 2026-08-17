// Package workspace 提供工作区文件浏览/读写/上传的 HTTP API。
package workspace

import (
	"io"
	"net/http"
	"os"
	"strings"

	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 工作区 API 控制器。
type Controller struct {
	service service.WorkspaceService
}

// NewController 创建工作区 API 控制器。
func NewController(service service.WorkspaceService) *Controller {
	return &Controller{service: service}
}

// ListTree 列出目录树。
// GET /api/v1/workspace/tree?path=agent-1
func (ctrl *Controller) ListTree(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}
	entries, err := ctrl.service.ListTree(c.Request.Context(), path)
	if err != nil {
		ctrl.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"entries": entries})
}

// ReadFile 读取文件内容（原始字节，按扩展名设 Content-Type）。
// GET /api/v1/workspace/file?path=agent-1/a.txt
func (ctrl *Controller) ReadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}
	data, err := ctrl.service.ReadContent(c.Request.Context(), path)
	if err != nil {
		ctrl.handleError(c, err)
		return
	}
	c.Data(http.StatusOK, contentTypeFor(path), data)
}

// SaveFile 保存文件内容。
// PUT /api/v1/workspace/file  body: {path, content}
func (ctrl *Controller) SaveFile(c *gin.Context) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Path == "" {
		response.BadRequest(c, "缺少 path")
		return
	}
	entry, err := ctrl.service.Save(c.Request.Context(), req.Path, []byte(req.Content))
	if err != nil {
		ctrl.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"entry": entry})
}

// DeleteFile 删除文件或目录。
// DELETE /api/v1/workspace/file?path=agent-1/a.txt
func (ctrl *Controller) DeleteFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}
	if err := ctrl.service.Delete(c.Request.Context(), path); err != nil {
		ctrl.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

// CreateDirectory 创建目录。
// POST /api/v1/workspace/directory  body: {parent_path, name}
func (ctrl *Controller) CreateDirectory(c *gin.Context) {
	var req struct {
		ParentPath string `json:"parent_path"`
		Name       string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Name == "" {
		response.BadRequest(c, "缺少 name")
		return
	}
	entry, err := ctrl.service.CreateDirectory(c.Request.Context(), req.ParentPath, req.Name)
	if err != nil {
		ctrl.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"entry": entry})
}

// Upload 上传文件（multipart）。
// POST /api/v1/workspace/upload  form: parent_path, files[]
func (ctrl *Controller) Upload(c *gin.Context) {
	parentPath := c.PostForm("parent_path")
	form, err := c.MultipartForm()
	if err != nil {
		response.BadRequest(c, "缺少上传文件")
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		response.BadRequest(c, "缺少 files 字段")
		return
	}
	uploads := make([]service.FileUpload, 0, len(files))
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			response.BadRequest(c, "读取上传文件失败: "+err.Error())
			return
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			response.BadRequest(c, "读取上传文件失败: "+err.Error())
			return
		}
		uploads = append(uploads, service.FileUpload{Name: fh.Filename, Content: data})
	}
	entries, err := ctrl.service.Upload(c.Request.Context(), parentPath, uploads)
	if err != nil {
		ctrl.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"entries": entries})
}

// handleError 统一错误处理：业务错误 → BizError；不存在 → 404；其余 → 400。
func (ctrl *Controller) handleError(c *gin.Context, err error) {
	if errors.IsBizError(err) {
		response.BizError(c, err)
		return
	}
	if os.IsNotExist(err) {
		response.NotFound(c, "文件不存在")
		return
	}
	logger.Error("工作区操作失败", zap.Error(err))
	response.BadRequest(c, err.Error())
}

// contentTypeFor 按扩展名推断 Content-Type。
func contentTypeFor(path string) string {
	switch {
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".md"):
		return "text/markdown; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".gif"):
		return "image/gif"
	case strings.HasSuffix(path, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(path, ".txt"), strings.HasSuffix(path, ".log"):
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
