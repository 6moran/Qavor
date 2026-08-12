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
}

// NewController 创建全局 RAG 设置控制器。
func NewController(settingsService service.RAGSettingsService) *Controller {
	return &Controller{settingsService: settingsService}
}

// RegisterRoutes 注册需要认证的系统设置路由。
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	settings := router.Group("/system/rag-settings", middleware.Auth())
	settings.GET("", ctrl.GetRAGSettings)
	settings.PUT("", ctrl.UpdateRAGSettings)
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

func toRAGSettingsResponse(settings *service.RAGSettings) *dto.RAGSettingsResponse {
	return &dto.RAGSettingsResponse{
		RerankModelID:   settings.RerankModelID,
		RerankModelName: settings.RerankModelName,
	}
}
