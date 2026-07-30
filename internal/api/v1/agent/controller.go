package agent

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 智能体控制器
type Controller struct {
	agentSvc service.AgentService
}

// NewController 创建智能体控制器
func NewController(agentSvc service.AgentService) *Controller {
	return &Controller{agentSvc: agentSvc}
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
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Get 获取智能体
func (ctrl *Controller) Get(c *gin.Context) {
	slug := c.Param("slug")
	resp, err := ctrl.agentSvc.GetAgent(slug)
	if err != nil {
		response.BizError(c, err)
		return
	}

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
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Delete 删除智能体
func (ctrl *Controller) Delete(c *gin.Context) {
	slug := c.Param("slug")
	if err := ctrl.agentSvc.DeleteAgent(slug); err != nil {
		response.BizError(c, err)
		return
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
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// SetDefault 设为默认智能体
func (ctrl *Controller) SetDefault(c *gin.Context) {
	slug := c.Param("slug")
	if err := ctrl.agentSvc.SetDefault(slug); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// GetDefault 获取默认智能体
func (ctrl *Controller) GetDefault(c *gin.Context) {
	resp, err := ctrl.agentSvc.GetDefaultAgent()
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}
