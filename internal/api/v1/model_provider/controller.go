package model_provider

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Controller 模型提供商控制器
// 负责处理模型提供商的 CRUD 操作的 HTTP 请求
type Controller struct {
	providerService service.ModelProviderService
}

// NewController 创建模型提供商控制器
// 参数:
//   - providerService: 模型提供商服务实例
//
// 返回: 模型提供商控制器实例
func NewController(providerService service.ModelProviderService) *Controller {
	return &Controller{providerService: providerService}
}

// RegisterRoutes 注册模型提供商的路由
// 在传入的路由组下注册 /model-providers 分组，包含所有 CRUD 端点
// 参数:
//   - router: Gin 路由组，通常是 /api/v1
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	providers := router.Group("/model-providers")
	{
		providers.POST("", ctrl.CreateProvider)
		providers.GET("", ctrl.ListProviders)
		providers.GET("/:id", ctrl.GetProvider)
		providers.PUT("/:id", ctrl.UpdateProvider)
		providers.DELETE("/:id", ctrl.DeleteProvider)
	}
}

// CreateProvider 创建模型提供商
// @Summary 创建模型提供商
// @Description 创建一个新的模型提供商配置
// @Tags 模型提供商
// @Accept json
// @Produce json
// @Param request body request.CreateModelProviderRequest true "模型提供商信息"
// @Success 200 {object} response.Response{data=response.ModelProviderResponse}
// @Router /api/v1/model-providers [post]
func (ctrl *Controller) CreateProvider(c *gin.Context) {
	var req request.CreateModelProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.providerService.CreateProvider(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetProvider 获取模型提供商详情
// @Summary 获取模型提供商
// @Description 根据 ID 获取模型提供商的详细信息
// @Tags 模型提供商
// @Accept json
// @Produce json
// @Param id path int true "模型提供商ID"
// @Success 200 {object} response.Response{data=response.ModelProviderResponse}
// @Router /api/v1/model-providers/{id} [get]
func (ctrl *Controller) GetProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	resp, err := ctrl.providerService.GetProvider(uint(id))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// UpdateProvider 更新模型提供商
// @Summary 更新模型提供商
// @Description 根据 ID 更新模型提供商的配置信息
// @Tags 模型提供商
// @Accept json
// @Produce json
// @Param id path int true "模型提供商ID"
// @Param request body request.UpdateModelProviderRequest true "更新信息"
// @Success 200 {object} response.Response{data=response.ModelProviderResponse}
// @Router /api/v1/model-providers/{id} [put]
func (ctrl *Controller) UpdateProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	var req request.UpdateModelProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.providerService.UpdateProvider(uint(id), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// DeleteProvider 删除模型提供商
// @Summary 删除模型提供商
// @Description 根据 ID 删除模型提供商
// @Tags 模型提供商
// @Accept json
// @Produce json
// @Param id path int true "模型提供商ID"
// @Success 200 {object} response.Response
// @Router /api/v1/model-providers/{id} [delete]
func (ctrl *Controller) DeleteProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	if err := ctrl.providerService.DeleteProvider(uint(id)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ListProviders 获取模型提供商列表
// @Summary 获取模型提供商列表
// @Description 分页获取模型提供商列表，支持关键词搜索
// @Tags 模型提供商
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(10)
// @Param keyword query string false "搜索关键词"
// @Success 200 {object} response.Response{data=response.ModelProviderListResponse}
// @Router /api/v1/model-providers [get]
func (ctrl *Controller) ListProviders(c *gin.Context) {
	var req request.ModelProviderListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.providerService.ListProviders(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}
