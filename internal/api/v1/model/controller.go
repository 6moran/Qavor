package model

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Controller 模型控制器
type Controller struct {
	modelService service.ModelService
}

// NewController 创建模型控制器
func NewController(modelService service.ModelService) *Controller {
	return &Controller{modelService: modelService}
}

// CreateModel 创建模型
// @Summary 创建模型
// @Description 创建一个新的模型配置
// @Tags 模型
// @Accept json
// @Produce json
// @Param request body request.CreateModelRequest true "模型信息"
// @Success 200 {object} response.Response{data=response.ModelResponse}
// @Router /api/v1/models [post]
func (ctrl *Controller) CreateModel(c *gin.Context) {
	var req request.CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.modelService.CreateModel(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetModel 获取模型详情
// @Summary 获取模型
// @Description 根据 ID 获取模型的详细信息
// @Tags 模型
// @Accept json
// @Produce json
// @Param id path int true "模型ID"
// @Success 200 {object} response.Response{data=response.ModelResponse}
// @Router /api/v1/models/{id} [get]
func (ctrl *Controller) GetModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	resp, err := ctrl.modelService.GetModel(uint(id))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// UpdateModel 更新模型
// @Summary 更新模型
// @Description 根据 ID 更新模型的配置信息
// @Tags 模型
// @Accept json
// @Produce json
// @Param id path int true "模型ID"
// @Param request body request.UpdateModelRequest true "更新信息"
// @Success 200 {object} response.Response{data=response.ModelResponse}
// @Router /api/v1/models/{id} [put]
func (ctrl *Controller) UpdateModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	var req request.UpdateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.modelService.UpdateModel(uint(id), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// DeleteModel 删除模型
// @Summary 删除模型
// @Description 根据 ID 删除模型
// @Tags 模型
// @Accept json
// @Produce json
// @Param id path int true "模型ID"
// @Success 200 {object} response.Response
// @Router /api/v1/models/{id} [delete]
func (ctrl *Controller) DeleteModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	if err := ctrl.modelService.DeleteModel(uint(id)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ListModels 获取模型列表
// @Summary 获取模型列表
// @Description 分页获取模型列表，支持关键词搜索和筛选
// @Tags 模型
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(10)
// @Param keyword query string false "搜索关键词"
// @Param model_type query string false "模型类型(chat/embedding/rerank)"
// @Success 200 {object} response.Response{data=response.ModelListResponse}
// @Router /api/v1/models [get]
func (ctrl *Controller) ListModels(c *gin.Context) {
	var req request.ModelListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.modelService.ListModels(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}
