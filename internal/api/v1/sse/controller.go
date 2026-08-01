package sse

import (
	"net/http"

	"Qavor/internal/middleware"
	"Qavor/internal/service"
	"Qavor/internal/sse"
	pkgerrors "Qavor/pkg/errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller SSE 控制器
type Controller struct {
	sseSvc  service.SSEService
	config  *sse.SSEConfig
	logger  *zap.Logger
}

// NewController 创建 SSE 控制器
func NewController(
	sseSvc service.SSEService,
	config *sse.SSEConfig,
	logger *zap.Logger,
) *Controller {
	return &Controller{
		sseSvc: sseSvc,
		config: config,
		logger: logger,
	}
}

// Stream 处理 SSE 流式请求
// POST /api/v1/chat/stream
func (ctrl *Controller) Stream(c *gin.Context) {
	// 1. 解析请求
	var req service.FileUploadRequest
	// 使用独立的请求结构体
	var streamReq struct {
		ConversationID uint   `json:"conversation_id" binding:"required"`
		Content        string `json:"content" binding:"required"`
		AgentSlug      string `json:"agent_slug"`
		ModelName      string `json:"model_name"`
		FileIDs        []uint `json:"file_ids"`
	}
	_ = req // 避免未使用警告

	if err := c.ShouldBindJSON(&streamReq); err != nil {
		c.JSON(http.StatusBadRequest, pkgerrors.New(pkgerrors.CodeSSEInvalidRequest, "请求参数无效: "+err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	taskID := sse.GenerateTaskID()

	// 2. 设置 SSE 响应头（必须在第一次写入前）
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 3. 创建 SSE 写入器（线程安全）
	writer := sse.NewSSEWriter(c, ctrl.logger)
	defer writer.Close()

	// 4. 调用 Service 处理流式对话
	err := ctrl.sseSvc.Stream(c.Request.Context(), &service.StreamRequest{
		TaskID:         taskID,
		UserID:         userID,
		ConversationID: streamReq.ConversationID,
		Content:        streamReq.Content,
		AgentSlug:      streamReq.AgentSlug,
		ModelName:      streamReq.ModelName,
		FileIDs:        streamReq.FileIDs,
		Writer:         writer,
	})

	if err != nil {
		ctrl.logger.Error("流式处理失败", zap.Error(err))
	}
}

// Cancel 取消正在生成的消息
// POST /api/v1/chat/cancel
func (ctrl *Controller) Cancel(c *gin.Context) {
	var req struct {
		TaskID string `json:"task_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, pkgerrors.New(pkgerrors.CodeSSEInvalidRequest, "请求参数无效: "+err.Error()))
		return
	}

	err := ctrl.sseSvc.Cancel(req.TaskID)
	if err != nil {
		c.JSON(http.StatusNotFound, pkgerrors.New(pkgerrors.CodeSSETaskNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "任务已取消",
	})
}

// UploadFile 上传文件
// POST /api/v1/chat/upload
func (ctrl *Controller) UploadFile(c *gin.Context) {
	var req service.FileUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, pkgerrors.New(pkgerrors.CodeSSEInvalidRequest, "请求参数无效: "+err.Error()))
		return
	}

	userID := middleware.GetUserID(c)

	resp, err := ctrl.sseSvc.UploadFile(userID, &req)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, pkgerrors.New(pkgerrors.CodeSSEFileTooLarge, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// ProcessFile 处理已上传的文件
// POST /api/v1/chat/process-file
func (ctrl *Controller) ProcessFile(c *gin.Context) {
	var req struct {
		FileID uint `json:"file_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, pkgerrors.New(pkgerrors.CodeSSEInvalidRequest, "请求参数无效: "+err.Error()))
		return
	}

	userID := middleware.GetUserID(c)

	err := ctrl.sseSvc.ProcessFile(userID, req.FileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, pkgerrors.New(pkgerrors.CodeSSEFileProcessFailed, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "文件处理请求已接受",
	})
}
