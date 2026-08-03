package mcp_server

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller MCP服务器控制器
type Controller struct {
	mcpServerService service.MCPServerService
}

// NewController 创建MCP服务器控制器
func NewController(mcpServerService service.MCPServerService) *Controller {
	return &Controller{
		mcpServerService: mcpServerService,
	}
}

// Create 创建MCP服务器
func (ctrl *Controller) Create(c *gin.Context) {
	var req request.CreateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.mcpServerService.CreateMCPServer(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Get 获取MCP服务器
func (ctrl *Controller) Get(c *gin.Context) {
	name := c.Param("name")

	resp, err := ctrl.mcpServerService.GetMCPServer(name)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Update 更新MCP服务器
func (ctrl *Controller) Update(c *gin.Context) {
	name := c.Param("name")

	var req request.UpdateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.mcpServerService.UpdateMCPServer(name, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Delete 删除MCP服务器
func (ctrl *Controller) Delete(c *gin.Context) {
	name := c.Param("name")

	if err := ctrl.mcpServerService.DeleteMCPServer(name); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// List 分页获取MCP服务器列表
func (ctrl *Controller) List(c *gin.Context) {
	var req request.MCPServerListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	pageResp, err := ctrl.mcpServerService.ListMCPServers(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, pageResp)
}

// Enable 启用MCP服务器
func (ctrl *Controller) Enable(c *gin.Context) {
	name := c.Param("name")

	if err := ctrl.mcpServerService.EnableMCPServer(name); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// Disable 停用MCP服务器
func (ctrl *Controller) Disable(c *gin.Context) {
	name := c.Param("name")

	if err := ctrl.mcpServerService.DisableMCPServer(name); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
