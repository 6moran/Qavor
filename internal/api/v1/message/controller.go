package message

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
		response.BadRequest(c, "无效的会话ID")
		return
	}

	var req request.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	req.ConversationID = uint(conversationID)

	resp, err := ctrl.messageService.CreateMessage(&req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，创建消息失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("创建消息失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, resp)
}

// GetMessage 获取消息详情
func (ctrl *Controller) GetMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的会话ID")
		return
	}

	idStr := c.Param("msg_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的消息ID")
		return
	}

	resp, err := ctrl.messageService.GetMessage(uint(id), uint(conversationID))
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取消息失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取消息失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, resp)
}

// UpdateMessage 更新消息
func (ctrl *Controller) UpdateMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的会话ID")
		return
	}

	idStr := c.Param("msg_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的消息ID")
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
		if errors.IsBizError(err) {
			logger.Warn("业务错误，更新消息失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("更新消息失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, resp)
}

// DeleteMessage 删除消息
func (ctrl *Controller) DeleteMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的会话ID")
		return
	}

	idStr := c.Param("msg_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的消息ID")
		return
	}

	if err := ctrl.messageService.DeleteMessage(uint(id), uint(conversationID)); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，删除消息失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("删除消息失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, nil)
}

// ListMessages 获取消息列表
func (ctrl *Controller) ListMessages(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的会话ID")
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
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取消息列表失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取消息列表失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, resp)
}

// GetLatestMessage 获取最新消息
func (ctrl *Controller) GetLatestMessage(c *gin.Context) {
	conversationIDStr := c.Param("id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的会话ID")
		return
	}

	resp, err := ctrl.messageService.GetLatestMessage(uint(conversationID))
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取最新消息失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取最新消息失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	response.Success(c, resp)
}
