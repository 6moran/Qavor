package chat

import (
	"context"

	"Qavor/internal/agent"
	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/entit
	"Qavor/internal/repository"
	"Qavor/internal/service"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// Controller 聊天控制器
type Controller struct {
	agentMgr        *agent.AgentManager
	modelSvc        service.ModelService
	messageRepo     repository.MessageRepository
	conversationSvc service.ConversationService
	messageSvc      service.MessageService
}

// NewController 创建聊天控制器
func NewController(
	agentMgr *agent.AgentManager,
	modelSvc service.ModelService,
	messageRepo repository.MessageRepository,
	conversationSvc service.ConversationService,
	messageSvc service.MessageService,
) *Controller {
	return &Controller{
		agentMgr:        agentMgr,
		modelSvc:        modelSvc,
		messageRepo:     messageRepo,
		conversationSvc: conversationSvc,
		messageSvc:      messageSvc,
	}
}

// ChatRequest 聊天请求
type ChatRequest struct {
	AgentSlug      string `json:"agent_slug"`
	ConversationID uint   `json:"conversation_id"` // 可选，不传则自动创建
	Message        string `json:"message" binding:"required"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	MessageID      uint   `json:"message_id"`
	ConversationID uint   `json:"conversation_id"`
	Content        string `json:"content"`
	DeliveryStatus string `json:"delivery_status"`
}

// Chat 聊天（异步）
func (ctrl *Controller) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// TODO: 从 JWT 上下文获取真实 user ID
	userID := uint(1)

	// 获取或创建会话
	conversationID := req.ConversationID
	if conversationID == 0 {
		conv, err := ctrl.conversationSvc.CreateConversation(userID, &request.CreateConversationRequest{
			AgentID: req.AgentSlug,
		})
		if err != nil {
			response.BizError(c, err)
			return
		}
		conversationID = conv.ID
	}

	// 保存用户消息
	_, err := ctrl.messageSvc.CreateMessage(userID, &request.CreateMessageRequest{
		ConversationID: conversationID,
		Role:           "user",
		Content:        req.Message,
	})
	if err != nil {
		response.BizError(c, err)
		return
	}

	// 创建占位符助手消息
	placeholder := &entity.Message{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        "🤔 正在准备连接工具...",
		MessageType:    "text",
		DeliveryStatus: "pending",
	}
	if err := ctrl.messageRepo.Create(placeholder); err != nil {
		response.BizError(c, err)
		return
	}

	// 立即返回
	response.Success(c, ChatResponse{
		MessageID:      placeholder.ID,
		ConversationID: conversationID,
		Content:        placeholder.Content,
		DeliveryStatus: "pending",
	})

	// 后台异步执行
	go ctrl.processMessage(c.Request.Context(), req.AgentSlug, placeholder.ID, req.Message)
}

// processMessage 后台处理消息
func (ctrl *Controller) processMessage(ctx context.Context, slug string, messageID uint, query string) {
	// 获取或创建 Agent（内部：缓存检查 → 查数据库 → 创建 → 缓存）
	a, err := ctrl.agentMgr.GetOrCreate(ctx, slug, nil)
	if err != nil {
		logger.Error("Agent 创建失败", zap.Error(err))
		ctrl.updateMessageStatus(messageID, "error", "Agent 创建失败: "+err.Error())
		return
	}

	// 执行 Agent
	resp, err := a.Execute(ctx, query)
	if err != nil {
		logger.Error("Agent 执行失败", zap.Error(err))
		ctrl.updateMessageStatus(messageID, "error", "执行失败: "+err.Error())
		return
	}

	// 更新占位符消息
	ctrl.updateMessageStatus(messageID, "complete", resp.Content)
}

// updateMessageStatus 更新消息内容和状态
func (ctrl *Controller) updateMessageStatus(messageID uint, status, content string) {
	msg, err := ctrl.messageRepo.FindByID(messageID)
	if err != nil || msg == nil {
		return
	}
	msg.Content = content
	msg.DeliveryStatus = status
	if err := ctrl.messageRepo.Update(msg); err != nil {
		logger.Error("更新消息状态失败", zap.Uint("messageID", messageID), zap.Error(err))
	}
}
