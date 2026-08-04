package conversation

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"
	"strconv"

	"github.com/gin-gonic/gin"
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
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetConversation 获取会话详情
func (ctrl *Controller) GetConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	resp, err := ctrl.conversationService.GetConversation(uint(id))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// UpdateConversation 更新会话
func (ctrl *Controller) UpdateConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
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
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// DeleteConversation 删除会话
func (ctrl *Controller) DeleteConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	if err := ctrl.conversationService.DeleteConversation(uint(id)); err != nil {
		response.BizError(c, err)
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
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// CloseConversation 关闭会话
func (ctrl *Controller) CloseConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	resp, err := ctrl.conversationService.CloseConversation(uint(id))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// ArchiveConversation 归档会话
func (ctrl *Controller) ArchiveConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	resp, err := ctrl.conversationService.ArchiveConversation(uint(id))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}
