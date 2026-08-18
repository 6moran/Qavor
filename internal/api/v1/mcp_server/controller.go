package mcp_server

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
		if errors.IsBizError(err) {
			logger.Warn("业务错误，创建MCP服务器失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("创建MCP服务器失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, resp)
}

// Get 获取MCP服务器
func (ctrl *Controller) Get(c *gin.Context) {
	name := c.Param("name")

	resp, err := ctrl.mcpServerService.GetMCPServer(name)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.InternalServerError(c)
		}
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
		if errors.IsBizError(err) {
			logger.Warn("业务错误，更新MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("更新MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, resp)
}

// Delete 删除MCP服务器
func (ctrl *Controller) Delete(c *gin.Context) {
	name := c.Param("name")

	if err := ctrl.mcpServerService.DeleteMCPServer(name); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，删除MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("删除MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.InternalServerError(c)
		}
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
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取MCP服务器列表失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取MCP服务器列表失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, pageResp)
}

// Enable 启用MCP服务器
func (ctrl *Controller) Enable(c *gin.Context) {
	name := c.Param("name")

	if err := ctrl.mcpServerService.EnableMCPServer(name); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，启用MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("启用MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, nil)
}

// Disable 停用MCP服务器
func (ctrl *Controller) Disable(c *gin.Context) {
	name := c.Param("name")

	if err := ctrl.mcpServerService.DisableMCPServer(name); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，停用MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("停用MCP服务器失败", zap.String("name", name), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, nil)
}

// Test 测试MCP服务器连接
func (ctrl *Controller) Test(c *gin.Context) {
	name := c.Param("name")

	if err := ctrl.mcpServerService.TestMCPServer(name); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，测试MCP服务器连接失败", zap.String("name", name), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("测试MCP服务器连接失败", zap.String("name", name), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, nil)
}

// TestConfig 测试表单中的 MCP 配置是否可连通
func (ctrl *Controller) TestConfig(c *gin.Context) {
	var req request.CreateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.mcpServerService.TestMCPServerConfig(&req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，测试MCP配置失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("测试MCP配置失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, resp)
}

// GetTools 获取MCP服务器的工具列表
func (ctrl *Controller) GetTools(c *gin.Context) {
	name := c.Param("name")

	tools, err := ctrl.mcpServerService.GetMCPServerTools(name)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取MCP服务器工具列表失败", zap.String("name", name), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取MCP服务器工具列表失败", zap.String("name", name), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, gin.H{"tools": tools})
}

// RefreshTools 刷新MCP服务器的工具列表
func (ctrl *Controller) RefreshTools(c *gin.Context) {
	name := c.Param("name")

	tools, err := ctrl.mcpServerService.RefreshMCPServerTools(name)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，刷新MCP服务器工具列表失败", zap.String("name", name), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("刷新MCP服务器工具列表失败", zap.String("name", name), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, gin.H{"tools": tools})
}
