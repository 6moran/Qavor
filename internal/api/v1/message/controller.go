package message

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Controller 消息控制器
type Controller struct {
	messageService service.MessageService
}

// NewController 创建消息控制器
func NewController(messageService service.MessageService) *Controller {
	return &Controller{messageService: messageService}
}

// CreateMessage 创建消息
func (ctrl *Controller) CreateMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	var req request.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	req.ConversationID = uint(conversationID)

	userID, _ := c.Get("user_id")
	resp, err := ctrl.messageService.CreateMessage(userID.(uint), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetMessage 获取消息详情
func (ctrl *Controller) GetMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	idStr := c.Param("msg_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的消息ID")
		return
	}

	resp, err := ctrl.messageService.GetMessage(uint(id), uint(conversationID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// UpdateMessage 更新消息
func (ctrl *Controller) UpdateMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	idStr := c.Param("msg_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的消息ID")
		return
	}

	var req request.UpdateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.messageService.UpdateMessage(uint(id), uint(conversationID), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// DeleteMessage 删除消息
func (ctrl *Controller) DeleteMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	idStr := c.Param("msg_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的消息ID")
		return
	}

	if err := ctrl.messageService.DeleteMessage(uint(id), uint(conversationID)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ListMessages 获取消息列表
func (ctrl *Controller) ListMessages(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	var req request.MessageListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.messageService.ListMessages(uint(conversationID), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetLatestMessage 获取最新消息
func (ctrl *Controller) GetLatestMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	resp, err := ctrl.messageService.GetLatestMessage(uint(conversationID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}
