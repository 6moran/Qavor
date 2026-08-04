package model

import (
	"Qavor/internal/llm"
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"
	"strconv"
	"strings"

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

// GetProviders 获取支持的供应商列表
// @Summary 获取供应商列表
// @Description 获取所有支持的模型供应商列表，供用户选择，支持关键词搜索和协议筛选
// @Tags 模型
// @Accept json
// @Produce json
// @Param keyword query string false "搜索关键词（名称或显示名）"
// @Param protocol query string false "协议类型筛选（openai/ollama）"
// @Success 200 {object} response.Response{data=[]llm.Provider}
// @Router /api/v1/models/providers [get]
func (ctrl *Controller) GetProviders(c *gin.Context) {
	keyword := c.Query("keyword")
	protocol := c.Query("protocol")

	result := llm.ProviderRegistry

	// 按协议筛选
	if protocol != "" {
		result = llm.GetProvidersByProtocol(protocol)
	}

	// 搜索供应商（不区分大小写）
	if keyword != "" {
		keyword = strings.ToLower(keyword)
		var filtered []llm.Provider
		for _, p := range result {
			if strings.Contains(strings.ToLower(p.Name), keyword) ||
				strings.Contains(strings.ToLower(p.DisplayName), keyword) {
				filtered = append(filtered, p)
			}
		}
		result = filtered
	}

	response.Success(c, result)
}

// GetProviderByName 根据供应商名称获取详情
// @Summary 获取供应商详情
// @Description 根据供应商名称获取配置详情，用于填充表单默认值
// @Tags 模型
// @Accept json
// @Produce json
// @Param name path string true "供应商名称（如 openai, deepseek）"
// @Success 200 {object} response.Response{data=llm.Provider}
// @Router /api/v1/models/providers/{name} [get]
func (ctrl *Controller) GetProviderByName(c *gin.Context) {
	name := c.Param("name")

	provider, found := llm.GetProviderByName(name)
	if !found {
		response.Error(c, 404, "供应商不存在")
		return
	}

	response.Success(c, provider)
}

// TestConnection 测试模型连接
// @Summary 测试模型连接
// @Description 在不保存配置的情况下验证 Chat/Embedding 模型是否可用
// @Tags 模型
// @Accept json
// @Produce json
// @Param request body request.ModelConnectionTestRequest true "连接测试请求"
// @Success 200 {object} response.Response{data=response.ModelConnectionTestResponse}
// @Router /api/v1/models/test [post]
func (ctrl *Controller) TestConnection(c *gin.Context) {
	var req request.ModelConnectionTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	result, err := ctrl.modelService.TestConnection(c.Request.Context(), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}
