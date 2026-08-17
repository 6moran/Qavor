package sse

import (
	"net/http"

	"Qavor/internal/middleware"
	"Qavor/internal/sse"
	pkgerrors "Qavor/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// 固定的管理员标识（无用户体系）
const adminID = "admin"

// Controller SSE 控制器
type Controller struct {
	manager *sse.Manager
	logger  *zap.Logger
}

// NewController 创建 SSE 控制器
func NewController(
	manager *sse.Manager,
	logger *zap.Logger,
) *Controller {
	return &Controller{
		manager: manager,
		logger:  logger,
	}
}

// Connect 建立 SSE 连接
// GET /api/v1/sse/connect
func (ctrl *Controller) Connect(c *gin.Context) {
	// 1. 验证认证（必须通过 Auth 中间件）
	if !middleware.IsAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, pkgerrors.New(pkgerrors.CodeUnauthorized, "未授权"))
		return
	}

	// 2. 获取设备ID（如果未提供则自动生成）
	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		deviceID = c.Query("device_id")
	}
	if deviceID == "" {
		deviceID = uuid.New().String()[:8]
	}

	// 3. 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 4. 创建 SSE 写入器
	writer := sse.NewSSEWriter(c, ctrl.logger)
	defer writer.Close()

	// 5. 建立连接（使用固定的 adminID）
	conn := ctrl.manager.Connect(c.Request.Context(), adminID, deviceID, writer)

	// 6. 等待连接关闭
	<-conn.Done

	ctrl.logger.Info("SSE 连接已关闭",
		zap.String("conn_id", conn.ConnID),
		zap.String("device_id", deviceID),
	)
}

// GetConnectionInfo 获取连接信息
// GET /api/v1/sse/info
func (ctrl *Controller) GetConnectionInfo(c *gin.Context) {
	if !middleware.IsAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, pkgerrors.New(pkgerrors.CodeUnauthorized, "未授权"))
		return
	}

	connections := ctrl.manager.GetConnections(adminID)
	count := len(connections)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"connection_count": count,
			"is_connected":     count > 0,
		},
	})
}
