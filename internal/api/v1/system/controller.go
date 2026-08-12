package system

import (
	"Qavor/internal/middleware"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 管理全局 RAG 设置接口。
type Controller struct {
	settingsService service.RAGSettingsService
	configService   service.SystemConfigService
}

// NewController 创建全局 RAG 设置控制器。
func NewController(settingsService service.RAGSettingsService, configService service.SystemConfigService) *Controller {
	return &Controller{settingsService: settingsService, configService: configService}
}

// RegisterRoutes 注册需要认证的系统设置路由。
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	settings := router.Group("/system/rag-settings", middleware.Auth())
	settings.GET("", ctrl.GetRAGSettings)
	settings.PUT("", ctrl.UpdateRAGSettings)

	config := router.Group("/system/config", middleware.Auth())
	config.GET("", ctrl.GetConfig)
	config.POST("", ctrl.UpdateConfig)
	config.POST("/update", ctrl.UpdateConfigBatch)
	config.GET("/options", ctrl.GetConfigOptions)
	config.PUT("/options/:key", ctrl.UpdateConfigOption)
}

// GetRAGSettings 返回当前全局 RAG 设置。
func (ctrl *Controller) GetRAGSettings(c *gin.Context) {
	settings, err := ctrl.settingsService.Get(c.Request.Context())
	if err != nil {
		ctrl.respondError(c, "读取全局 RAG 设置失败", err)
		return
	}
	response.Success(c, toRAGSettingsResponse(settings))
}

// UpdateRAGSettings 更新全局重排模型设置。
func (ctrl *Controller) UpdateRAGSettings(c *gin.Context) {
	var req request.UpdateRAGSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	settings, err := ctrl.settingsService.UpdateRerankModel(c.Request.Context(), req.RerankModelID)
	if err != nil {
		ctrl.respondError(c, "更新全局 RAG 设置失败", err)
		return
	}
	response.Success(c, toRAGSettingsResponse(settings))
}

func (ctrl *Controller) respondError(c *gin.Context, message string, err error) {
	if errors.IsBizError(err) {
		logger.Warn(message, zap.Error(err))
		response.BizError(c, err)
		return
	}
	logger.Error(message, zap.Error(err))
	response.InternalServerError(c)
}

// GetConfig 返回全部系统配置（默认模型、内容审查等）。
func (ctrl *Controller) GetConfig(c *gin.Context) {
	cfg, err := ctrl.configService.Get(c.Request.Context())
	if err != nil {
		ctrl.respondError(c, "读取系统配置失败", err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfig 更新单个系统配置项并返回最新配置。
func (ctrl *Controller) UpdateConfig(c *gin.Context) {
	var req request.UpdateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	cfg, err := ctrl.configService.Update(c.Request.Context(), req.Key, req.Value)
	if err != nil {
		ctrl.respondError(c, "更新系统配置失败", err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfigBatch 批量更新系统配置项并返回最新配置。
func (ctrl *Controller) UpdateConfigBatch(c *gin.Context) {
	var values map[string]any
	if err := c.ShouldBindJSON(&values); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	cfg, err := ctrl.configService.UpdateBatch(c.Request.Context(), values)
	if err != nil {
		ctrl.respondError(c, "更新系统配置失败", err)
		return
	}
	response.Success(c, cfg)
}

// GetConfigOptions 返回全部可配置项定义（OCR 服务配置等，前端表单渲染用）。
func (ctrl *Controller) GetConfigOptions(c *gin.Context) {
	options, err := ctrl.configService.GetConfigOptions(c.Request.Context())
	if err != nil {
		ctrl.respondError(c, "读取配置项失败", err)
		return
	}
	response.Success(c, gin.H{"options": options})
}

// UpdateConfigOption 更新单个配置项并返回更新后的定义。
func (ctrl *Controller) UpdateConfigOption(c *gin.Context) {
	var req request.UpdateConfigOptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	option, err := ctrl.configService.UpdateConfigOption(c.Request.Context(), c.Param("key"), req.Value)
	if err != nil {
		ctrl.respondError(c, "更新配置项失败", err)
		return
	}
	response.Success(c, gin.H{"option": option})
}

func toRAGSettingsResponse(settings *service.RAGSettings) *dto.RAGSettingsResponse {
	return &dto.RAGSettingsResponse{
		RerankModelID:   settings.RerankModelID,
		RerankModelName: settings.RerankModelName,
	}
}
