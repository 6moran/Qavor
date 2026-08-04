package conversation

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

// Controller 会话控制器
type Controller struct {
	conversationService service.ConversationService
}

// NewController 创建会话控制器
func NewController(conversationService service.ConversationService) *Controller {
	return &Controller{conversationService: conversationService}
}

// CreateConversation 创建会话
func (ctrl *Controller) CreateConversation(c *gin.Context) {
	var req request.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.conversationService.CreateConversation(&req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，创建会话失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("创建会话失败", zap.Error(err))
			response.InternalError(c, "服务器内部错误")
		}
		return
	}

	response.Success(c, resp)
}

// GetConversation 获取会话详情
func (ctrl *Controller) GetConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	resp, err := ctrl.conversationService.GetConversation(uint(id))
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取会话失败", zap.Uint64("id", id), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取会话失败", zap.Uint64("id", id), zap.Error(err))
			response.InternalError(c, "服务器内部错误")
		}
		return
	}

	response.Success(c, resp)
}

// UpdateConversation 更新会话
func (ctrl *Controller) UpdateConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	var req request.UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.conversationService.UpdateConversation(uint(id), &req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，更新会话失败", zap.Uint64("id", id), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("更新会话失败", zap.Uint64("id", id), zap.Error(err))
			response.InternalError(c, "服务器内部错误")
		}
		return
	}

	response.Success(c, resp)
}

// DeleteConversation 删除会话
func (ctrl *Controller) DeleteConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := ctrl.conversationService.DeleteConversation(uint(id)); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，删除会话失败", zap.Uint64("id", id), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("删除会话失败", zap.Uint64("id", id), zap.Error(err))
			response.InternalError(c, "服务器内部错误")
		}
		return
	}

	response.Success(c, nil)
}

// ListConversations 获取会话列表
func (ctrl *Controller) ListConversations(c *gin.Context) {
	var req request.ConversationListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.conversationService.ListConversations(&req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取会话列表失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取会话列表失败", zap.Error(err))
			response.InternalError(c, "服务器内部错误")
		}
		return
	}

	response.Success(c, resp)
}

// CloseConversation 关闭会话
func (ctrl *Controller) CloseConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	resp, err := ctrl.conversationService.CloseConversation(uint(id))
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，关闭会话失败", zap.Uint64("id", id), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("关闭会话失败", zap.Uint64("id", id), zap.Error(err))
			response.InternalError(c, "服务器内部错误")
		}
		return
	}

	response.Success(c, resp)
}

// ArchiveConversation 归档会话
func (ctrl *Controller) ArchiveConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	resp, err := ctrl.conversationService.ArchiveConversation(uint(id))
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，归档会话失败", zap.Uint64("id", id), zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("归档会话失败", zap.Uint64("id", id), zap.Error(err))
			response.InternalError(c, "服务器内部错误")
		}
		return
	}

	response.Success(c, resp)
}
