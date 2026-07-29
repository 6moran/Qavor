// Package knowledge_base 提供知识库元数据 CRUD 的 HTTP API。
package knowledge_base

import (
	"Qavor/internal/middleware"
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 负责知识库请求参数绑定、业务服务调用和统一响应转换
type Controller struct{ service service.KnowledgeBaseService }

// NewController 创建知识库 API 控制器
func NewController(service service.KnowledgeBaseService) *Controller {
	return &Controller{service: service}
}

// Create 创建知识库
func (ctrl *Controller) Create(c *gin.Context) {
	var req request.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.Create(&req, middleware.GetUsername(c))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// List 获取知识库列表
func (ctrl *Controller) List(c *gin.Context) {
	var req request.KnowledgeBaseListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.List(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// Get 根据路径中的 kb_id 获取知识库详情
func (ctrl *Controller) Get(c *gin.Context) {
	result, err := ctrl.service.Get(c.Param("kb_id"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// Update 根据 kb_id 更新知识库元数据
func (ctrl *Controller) Update(c *gin.Context) {
	var req request.UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.Update(c.Param("kb_id"), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// Delete 根据 kb_id 删除知识库记录
func (ctrl *Controller) Delete(c *gin.Context) {
	if err := ctrl.service.Delete(c.Param("kb_id")); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}
