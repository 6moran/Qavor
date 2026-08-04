package chat

import (
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 聊天控制器
type Controller struct {
	chatSvc service.ChatService
}

// NewController 创建聊天控制器
func NewController(
	chatSvc service.ChatService,
) *Controller {
	return &Controller{
		chatSvc: chatSvc,
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

// Chat 聊天
func (ctrl *Controller) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 调用 ChatService
	result, err := ctrl.chatSvc.Chat(c.Request.Context(), req.ConversationID, req.AgentSlug, req.Message)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，聊天失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("聊天失败", zap.Error(err))
			response.InternalError(c, "服务器内部错误")
		}
		return
	}

	response.Success(c, ChatResponse{
		MessageID:      result.MessageID,
		ConversationID: result.ConversationID,
		Content:        result.Content,
		DeliveryStatus: result.DeliveryStatus,
	})
}
