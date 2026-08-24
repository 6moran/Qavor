package chat

import (
	"encoding/json"
	"fmt"
	"strconv"

	"Qavor/internal/service"
	"Qavor/internal/trace"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 聊天控制器
type Controller struct {
	chatSvc service.ChatService
	tracer  *trace.Tracer
}

// NewController 创建聊天控制器
func NewController(
	chatSvc service.ChatService,
	tracer *trace.Tracer,
) *Controller {
	return &Controller{
		chatSvc: chatSvc,
		tracer:  tracer,
	}
}

// ChatRequest 聊天请求
type ChatRequest struct {
	AgentSlug      string          `json:"agent_slug"`
	ConversationID json.RawMessage `json:"conversation_id"` // 可选，支持数字或字符串
	Message        string          `json:"message" binding:"required"`
}

// parseConversationID 解析 conversation_id，支持数字和字符串
func parseConversationID(raw json.RawMessage) (uint, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "0" {
		return 0, nil
	}

	// 尝试解析为数字
	var num uint
	if err := json.Unmarshal(raw, &num); err == nil {
		return num, nil
	}

	// 尝试解析为字符串再转数字
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if id, err := strconv.ParseUint(str, 10, 32); err == nil {
			return uint(id), nil
		}
	}

	return 0, fmt.Errorf("无效的 conversation_id")
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

	conversationID, err := parseConversationID(req.ConversationID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if ctrl.tracer != nil {
		ctrl.tracer.UpdateRequestMetadata(c.Request.Context(), conversationID, req.Message, "sync")
	}

	// 调用 ChatService
	result, err := ctrl.chatSvc.Chat(c.Request.Context(), conversationID, req.AgentSlug, req.Message)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，聊天失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("聊天失败", zap.Error(err))
			response.InternalServerErrorWithDetail(c, err)
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
