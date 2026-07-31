package tool

import (
	"Qavor/internal/tool"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 工具控制器
type Controller struct {
	registry *tool.Registry
}

// NewController 创建工具控制器
func NewController(registry *tool.Registry) *Controller {
	return &Controller{registry: registry}
}

// GetTools 获取工具列表
func (c *Controller) GetTools(ctx *gin.Context) {
	category := ctx.Query("category")

	var tools []tool.ToolMeta
	if category != "" {
		tools = c.registry.ListByCategory(tool.Category(category))
	} else {
		tools = c.registry.ListAll()
	}

	response.Success(ctx, gin.H{
		"tools": tools,
	})
}

// GetToolOptions 获取工具选项（用于 Agent 配置页）
func (c *Controller) GetToolOptions(ctx *gin.Context) {
	tools := c.registry.ListAll()

	// 构建分类列表
	categories := []gin.H{
		{"name": "builtin", "label": "系统工具"},
		{"name": "knowledge", "label": "知识库工具"},
		{"name": "platform", "label": "平台工具"},
	}

	// 构建工具列表（带选中状态）
	toolOptions := make([]gin.H, 0, len(tools))
	for _, t := range tools {
		toolOptions = append(toolOptions, gin.H{
			"name":        t.Name,
			"description": t.Description,
			"category":    t.Category,
			"selected":    false,
		})
	}

	response.Success(ctx, gin.H{
		"categories": categories,
		"tools":      toolOptions,
	})
}
