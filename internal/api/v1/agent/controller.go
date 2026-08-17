package agent

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OptionsProvider 提供 configurable_items 动态 options 的数据源接口。
// 由 app.go 装配具体实现，避免 controller 直接依赖多个 service 造成循环依赖。
type OptionsProvider interface {
	// ToolOptions 返回内置工具选项
	ToolOptions() []map[string]interface{}
	// MCPServerOptions 返回 MCP 服务器选项
	MCPServerOptions() []map[string]interface{}
	// SkillOptions 返回技能选项
	SkillOptions() []map[string]interface{}
	// KnowledgeBaseOptions 返回知识库选项
	KnowledgeBaseOptions() []map[string]interface{}
	// SubagentOptions 返回子智能体选项
	SubagentOptions() []map[string]interface{}
}

// AgentCacheInvalidator 清除 Agent 运行时缓存。
// 由 app.go 装配 AgentManager 实现，避免 controller 直接依赖 agent 包造成耦合。
type AgentCacheInvalidator interface {
	// ClearAgentCache 清除指定 Agent 的运行时缓存，强制下次请求按最新配置重建
	ClearAgentCache(slug string)
	// InvalidateBySubagent 子智能体配置变更/增删时，失效所有引用它的主智能体
	InvalidateBySubagent(slug string)
}

// Controller 智能体控制器
type Controller struct {
	agentSvc         service.AgentService
	opts             OptionsProvider
	cacheInvalidator AgentCacheInvalidator
}

// NewController 创建智能体控制器
func NewController(agentSvc service.AgentService, opts OptionsProvider, cacheInvalidator AgentCacheInvalidator) *Controller {
	return &Controller{
		agentSvc:         agentSvc,
		opts:             opts,
		cacheInvalidator: cacheInvalidator,
	}
}

// enrichOptions 为 AgentResponse 的 configurable_items 填充动态 options
func (ctrl *Controller) enrichOptions(resp *dto.AgentResponse) {
	if resp == nil || resp.ConfigurableItems == nil || ctrl.opts == nil {
		return
	}
	if item, ok := resp.ConfigurableItems["tools"]; ok {
		item.Options = ctrl.opts.ToolOptions()
		resp.ConfigurableItems["tools"] = item
	}
	if item, ok := resp.ConfigurableItems["mcp_servers"]; ok {
		item.Options = ctrl.opts.MCPServerOptions()
		resp.ConfigurableItems["mcp_servers"] = item
	}
	if item, ok := resp.ConfigurableItems["skills"]; ok {
		item.Options = ctrl.opts.SkillOptions()
		resp.ConfigurableItems["skills"] = item
	}
	if item, ok := resp.ConfigurableItems["knowledges"]; ok {
		item.Options = ctrl.opts.KnowledgeBaseOptions()
		resp.ConfigurableItems["knowledges"] = item
	}
	if item, ok := resp.ConfigurableItems["subagents"]; ok {
		item.Options = ctrl.opts.SubagentOptions()
		resp.ConfigurableItems["subagents"] = item
	}
}

// Create 创建智能体
func (ctrl *Controller) Create(c *gin.Context) {
	var req request.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.agentSvc.CreateAgent(&req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，创建智能体失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("创建智能体失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	ctrl.enrichOptions(resp)
	response.Success(c, resp)
}

// Get 获取智能体
func (ctrl *Controller) Get(c *gin.Context) {
	slug := c.Param("slug")
	resp, err := ctrl.agentSvc.GetAgent(slug)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取智能体失败", zap.String("slug", slug), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取智能体失败", zap.String("slug", slug), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	ctrl.enrichOptions(resp)
	response.Success(c, resp)
}

// Update 更新智能体
func (ctrl *Controller) Update(c *gin.Context) {
	slug := c.Param("slug")
	var req request.UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.agentSvc.UpdateAgent(slug, &req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，更新智能体失败", zap.String("slug", slug), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("更新智能体失败", zap.String("slug", slug), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	ctrl.enrichOptions(resp)

	// 配置更新后清除运行时缓存：GetOrCreate 命中旧缓存时，
	// skill 列表、tools 等配置变更不会反映到已创建的 Agent 上。
	// 同时反向失效引用它的主智能体（本 agent 若作为子智能体被挂载）。
	if ctrl.cacheInvalidator != nil {
		ctrl.cacheInvalidator.ClearAgentCache(slug)
		ctrl.cacheInvalidator.InvalidateBySubagent(slug)
	}

	response.Success(c, resp)
}

// Delete 删除智能体
func (ctrl *Controller) Delete(c *gin.Context) {
	slug := c.Param("slug")
	if err := ctrl.agentSvc.DeleteAgent(slug); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，删除智能体失败", zap.String("slug", slug), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("删除智能体失败", zap.String("slug", slug), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	// 删除后反向失效引用它的主智能体（若该 agent 作为子智能体被挂载），
	// 使下次主智能体重建时读取最新配置（该子已被移除）。
	if ctrl.cacheInvalidator != nil {
		ctrl.cacheInvalidator.InvalidateBySubagent(slug)
	}

	response.Success(c, nil)
}

// List 智能体列表
func (ctrl *Controller) List(c *gin.Context) {
	var req request.AgentListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.agentSvc.ListAgents(&req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取智能体列表失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取智能体列表失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	// 分页响应的 List 是 []dto.AgentResponse，逐个填充 options
	if resp != nil {
		if items, ok := resp.List.([]dto.AgentResponse); ok {
			for i := range items {
				ctrl.enrichOptions(&items[i])
			}
			resp.List = items
		}
	}

	response.Success(c, resp)
}

// SetDefault 设为默认智能体
func (ctrl *Controller) SetDefault(c *gin.Context) {
	slug := c.Param("slug")
	if err := ctrl.agentSvc.SetDefault(slug); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，设置默认智能体失败", zap.String("slug", slug), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("设置默认智能体失败", zap.String("slug", slug), zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, nil)
}

// GetDefault 获取默认智能体
func (ctrl *Controller) GetDefault(c *gin.Context) {
	resp, err := ctrl.agentSvc.GetDefaultAgent()
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取默认智能体失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取默认智能体失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	ctrl.enrichOptions(resp)
	response.Success(c, resp)
}
