package chat

import (
	"Qavor/internal/agent"
	"Qavor/internal/service"
	"Qavor/pkg/response"

	"github.com/cloudwego/eino/components/model"
	"github.com/gin-gonic/gin"
)

// Controller 聊天控制器
type Controller struct {
	agentMgr    *agent.AgentManager
	agentSvc    service.AgentService
	providerSvc service.ModelProviderService
}

// NewController 创建聊天控制器
func NewController(agentMgr *agent.AgentManager, agentSvc service.AgentService, providerSvc service.ModelProviderService) *Controller {
	return &Controller{
		agentMgr:    agentMgr,
		agentSvc:    agentSvc,
		providerSvc: providerSvc,
	}
}

// ChatRequest 聊天请求
type ChatRequest struct {
	AgentSlug string `json:"agent_slug"`
	Message   string `json:"message" binding:"required"`
}

// Chat 聊天
func (ctrl *Controller) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 获取 agent 配置
	slug := req.AgentSlug
	var cfg *agent.AgentConfig
	var err error

	if slug == "" {
		defaultResp, e := ctrl.agentSvc.GetDefaultAgent()
		if e != nil {
			response.BizError(c, e)
			return
		}
		slug = defaultResp.Slug
		cfg = &defaultResp.Config
	} else {
		cfg, err = ctrl.agentSvc.GetAgentConfig(slug)
		if err != nil {
			response.BizError(c, err)
			return
		}
	}

	// 根据 agent 配置获取 LLM
	var llm model.ToolCallingChatModel
	if cfg.ProviderID != "" && cfg.ModelName != "" {
		m, e := ctrl.providerSvc.GetToolCallingModel(c.Request.Context(), cfg.ProviderID, cfg.ModelName)
		if e != nil {
			response.BizError(c, e)
			return
		}
		llm = m
	}

	// 创建 agent
	a, err := ctrl.agentMgr.Create(c.Request.Context(), cfg, llm)
	if err != nil {
		response.BizError(c, err)
		return
	}

	reply, err := a.Chat(c.Request.Context(), req.Message)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{
		"reply": reply,
	})
}
