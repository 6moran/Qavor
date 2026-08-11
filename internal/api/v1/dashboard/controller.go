package dashboard

import (
	"Qavor/internal/service"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 仪表盘控制器
type Controller struct {
	dashboardService service.DashboardService
}

// NewController 创建仪表盘控制器
func NewController(dashboardService service.DashboardService) *Controller {
	return &Controller{dashboardService: dashboardService}
}

// GetCallTimeseries 获取调用统计时间序列
// GET /api/v1/dashboard/stats/calls/timeseries?type=models&time_range=7days
func (ctrl *Controller) GetCallTimeseries(c *gin.Context) {
	dataType := c.DefaultQuery("type", "models")
	timeRange := c.DefaultQuery("time_range", "7days")

	validTypes := map[string]bool{"models": true, "agents": true, "tokens": true}
	validRanges := map[string]bool{"today": true, "7days": true, "thisMonth": true}

	if !validTypes[dataType] {
		response.BadRequest(c, "无效的数据类型，支持: models/agents/tokens")
		return
	}
	if !validRanges[timeRange] {
		response.BadRequest(c, "无效的时间范围，支持: today/7days/thisMonth")
		return
	}

	result, err := ctrl.dashboardService.GetCallTimeseries(c.Request.Context(), dataType, timeRange)
	if err != nil {
		logger.Error("获取调用统计数据失败", zap.String("type", dataType), zap.String("time_range", timeRange), zap.Error(err))
		response.InternalServerError(c)
		return
	}

	response.Success(c, result)
}
