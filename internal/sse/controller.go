package sse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	contextmgr "Qavor/internal/context"
	"Qavor/internal/llm"
	"Qavor/internal/middleware"
	"Qavor/internal/repository"
	"Qavor/internal/service"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Controller SSE 控制器
type Controller struct {
	contextMgr  contextmgr.ContextManager
	messageRepo repository.MessageRepository
	messageSvc  service.MessageService
	llmFactory  llm.ClientFactory
	config      *SSEConfig
	logger      *zap.Logger
	heartbeat   *HeartbeatManager

	// 任务管理：支持取消
	mu          sync.RWMutex
	activeTasks map[string]*TaskInfo // taskID -> TaskInfo

	// 会话任务管理：同一会话串行处理
	conversationTasks map[uint]string // conversationID -> taskID

	// 用户任务计数：优化并发检查 O(1) 复杂度
	userTaskCount map[uint]int // userID -> 当前任务数
}

// TaskInfo 任务信息
type TaskInfo struct {
	Cancel         context.CancelFunc
	UserID         uint
	TaskID         string
	ConversationID uint
}

// NewController 创建 SSE 控制器
func NewController(
	contextMgr contextmgr.ContextManager,
	messageRepo repository.MessageRepository,
	messageSvc service.MessageService,
	llmFactory llm.ClientFactory,
	config *SSEConfig,
	logger *zap.Logger,
) *Controller {
	return &Controller{
		contextMgr:       contextMgr,
		messageRepo:      messageRepo,
		messageSvc:       messageSvc,
		llmFactory:       llmFactory,
		config:           config,
		logger:           logger,
		heartbeat:        NewHeartbeatManager(config.HeartbeatInterval, logger),
		activeTasks:      make(map[string]*TaskInfo),
		conversationTasks: make(map[uint]string),
		userTaskCount:    make(map[uint]int),
	}
}

// GenerateTaskID 生成任务ID
func GenerateTaskID() string {
	id := uuid.New().String()[:8]
	timestamp := time.Now().Unix() % 1000000
	return fmt.Sprintf("task_%s_%06d", id, timestamp)
}

// Stream 处理 SSE 流式请求
// POST /api/v1/chat/stream
func (ctrl *Controller) Stream(c *gin.Context) {
	// 1. 解析请求
	var req StreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(GetHTTPStatus(ErrCodeInvalidRequest), "请求参数无效: "+err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	conversationID := req.ConversationID
	taskID := GenerateTaskID()

	// 2. 检查用户并发任务限制
	if err := ctrl.checkUserTaskLimit(userID); err != nil {
		c.JSON(http.StatusTooManyRequests, NewErrorResponse(GetHTTPStatus(ErrCodeTooManyRequests), err.Error()))
		return
	}

	// 3. 校验会话权限
	// TODO: 调用 ConversationService 校验会话属于当前用户

	// 4. 取消会话的当前任务（同一会话串行处理）
	ctrl.cancelConversationTask(conversationID)

	// 5. 设置 SSE 响应头（必须在第一次写入前）
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 6. 创建 SSE 写入器（线程安全）
	writer := NewSSEWriter(c, ctrl.logger)
	defer writer.Close()

	// 7. 创建任务上下文（支持取消和超时）
	ctx, cancel := context.WithTimeout(c.Request.Context(), ctrl.config.MaxStreamTime)
	defer cancel()

	// 注册任务（支持外部取消）
	ctrl.registerTask(taskID, cancel, userID, conversationID)
	defer ctrl.unregisterTask(taskID, conversationID)

	// 记录日志：请求接收
	ctrl.logger.Info("收到流式请求",
		zap.String("task_id", taskID),
		zap.Uint("user_id", userID),
		zap.Uint("conversation_id", conversationID),
		zap.String("model", req.ModelName),
	)

	// 8. 构建上下文
	query := &contextmgr.ContextHistoryQuery{
		ConversationID: conversationID,
		Limit:          50,
	}
	window, err := ctrl.contextMgr.FetchContext(ctx, query)
	if err != nil {
		ctrl.logger.Error("构建上下文失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		writer.Send(EventMessageError, ErrorData{
			Code:    ErrCodeContextBuildFailed,
			Message: "构建上下文失败",
		})
		writer.Send(EventDone, nil)
		return
	}

	// 记录日志：上下文构建完成
	ctrl.logger.Info("上下文构建完成",
		zap.String("task_id", taskID),
		zap.Int("history_count", len(window.Messages)),
	)

	// 9. 创建用户消息并持久化
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: req.Content,
	}
	userMsgID, err := ctrl.contextMgr.PersistUserMessage(ctx, conversationID, userMessage)
	if err != nil {
		ctrl.logger.Error("保存用户消息失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		// 不阻塞流程，继续执行
	}
	_ = userMsgID

	// 10. 组装 Prompt
	messages := ctrl.contextMgr.BuildPrompt(ctx, window, userMessage)

	// 11. 获取 LLM 客户端
	llmClient, err := ctrl.getLLMClient(req.AgentSlug, req.ModelName)
	if err != nil {
		ctrl.logger.Error("LLM 初始化失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		writer.Send(EventMessageError, ErrorData{
			Code:    ErrCodeLLMInitFailed,
			Message: "初始化LLM失败: " + err.Error(),
		})
		writer.Send(EventDone, nil)
		return
	}

	// 12. 生成消息ID
	messageID := fmt.Sprintf("msg_%s", uuid.New().String()[:8])

	// 13. 启动心跳
	stopHeartbeat := ctrl.heartbeat.Start(ctx, writer, messageID)
	defer ctrl.heartbeat.Stop(stopHeartbeat)

	// 14. 发送 message.start 事件
	writer.Send(EventMessageStart, MessageStartData{
		MessageID:      messageID,
		ConversationID: conversationID,
		Model:          req.ModelName,
	})

	// 记录日志：LLM 调用开始
	ctrl.logger.Info("LLM 流式调用开始",
		zap.String("task_id", taskID),
		zap.String("message_id", messageID),
		zap.String("model", req.ModelName),
	)

	// 15. 调用 LLM 流式接口
	streamReader, err := llmClient.Stream(ctx, messages)
	if err != nil {
		ctrl.logger.Error("LLM 流式调用失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		writer.Send(EventMessageError, ErrorData{
			Code:    ErrCodeLLMStreamFailed,
			Message: "LLM调用失败: " + err.Error(),
		})
		writer.Send(EventDone, nil)
		return
	}
	defer streamReader.Close()

	// 16. 读取流式输出并推送事件
	var fullContent string
	var index int
	startTime := time.Now()

	for {
		chunk, err := streamReader.Recv()
		if err != nil {
			// 检查是否是正常结束
			if errors.Is(err, io.EOF) {
				break
			}
			// 检查是否是取消
			if ctx.Err() == context.Canceled {
				ctrl.logger.Info("任务已取消",
					zap.String("task_id", taskID),
					zap.String("message_id", messageID),
				)
				writer.Send(EventMessageCancelled, MessageCancelledData{
					MessageID: messageID,
					Reason:    "user_cancel",
				})
				writer.Send(EventDone, nil)
				return
			}
			// 检查是否是超时
			if ctx.Err() == context.DeadlineExceeded {
				ctrl.logger.Warn("任务超时",
					zap.String("task_id", taskID),
					zap.String("message_id", messageID),
				)
				writer.Send(EventMessageError, ErrorData{
					Code:    ErrCodeTimeout,
					Message: "请求超时",
				})
				writer.Send(EventDone, nil)
				return
			}
			// 其他错误
			ctrl.logger.Error("读取流式输出失败",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			writer.Send(EventMessageError, ErrorData{
				Code:    ErrCodeStreamReadFailed,
				Message: "读取流式输出失败",
			})
			writer.Send(EventDone, nil)
			return
		}

		// 推送 message.delta 事件
		if chunk != nil && chunk.Content != "" {
			fullContent += chunk.Content
			writer.Send(EventMessageDelta, MessageDeltaData{
				MessageID: messageID,
				Content:   chunk.Content,
				Index:     index,
			})
			index++
		}
	}

	// 17. 持久化 Assistant 消息
	assistantMessage := &schema.Message{
		Role:    schema.Assistant,
		Content: fullContent,
	}
	if err := ctrl.contextMgr.PersistAssistantMessage(ctx, conversationID, assistantMessage); err != nil {
		ctrl.logger.Error("保存Assistant消息失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
	}

	// 18. 发送 message.complete 事件
	duration := time.Since(startTime)
	writer.Send(EventMessageComplete, MessageCompleteData{
		MessageID:    messageID,
		Content:      fullContent,
		TokenCount:   0, // TODO: 从LLM响应中获取
		FinishReason: "stop",
	})

	// 记录日志：流式完成
	ctrl.logger.Info("流式输出完成",
		zap.String("task_id", taskID),
		zap.String("message_id", messageID),
		zap.Duration("duration", duration),
	)

	// 19. 发送 done 事件
	writer.Send(EventDone, nil)
}

// Cancel 取消正在生成的消息
// POST /api/v1/chat/cancel
func (ctrl *Controller) Cancel(c *gin.Context) {
	var req CancelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(GetHTTPStatus(ErrCodeInvalidRequest), "请求参数无效: "+err.Error()))
		return
	}

	ctrl.mu.RLock()
	taskInfo, ok := ctrl.activeTasks[req.TaskID]
	ctrl.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, NewErrorResponse(GetHTTPStatus(ErrCodeTaskNotFound), "任务不存在或已完成"))
		return
	}

	// 记录日志
	ctrl.logger.Info("用户取消任务",
		zap.String("task_id", req.TaskID),
		zap.Uint("user_id", taskInfo.UserID),
		zap.Uint("conversation_id", taskInfo.ConversationID),
	)

	// 取消任务
	taskInfo.Cancel()

	c.JSON(http.StatusOK, NewSuccessResponse(map[string]string{"message": "任务已取消"}))
}

// registerTask 注册活跃任务
func (ctrl *Controller) registerTask(taskID string, cancel context.CancelFunc, userID uint, conversationID uint) {
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()
	ctrl.activeTasks[taskID] = &TaskInfo{
		Cancel:         cancel,
		UserID:         userID,
		TaskID:         taskID,
		ConversationID: conversationID,
	}
	// 记录会话的当前任务
	ctrl.conversationTasks[conversationID] = taskID
	// 增加用户任务计数
	ctrl.userTaskCount[userID]++
}

// unregisterTask 注销活跃任务
func (ctrl *Controller) unregisterTask(taskID string, conversationID uint) {
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	if task, ok := ctrl.activeTasks[taskID]; ok {
		// 减少用户任务计数
		ctrl.userTaskCount[task.UserID]--
		if ctrl.userTaskCount[task.UserID] <= 0 {
			delete(ctrl.userTaskCount, task.UserID)
		}
	}

	delete(ctrl.activeTasks, taskID)

	// 如果当前会话的任务就是这个任务，清除映射
	if ctrl.conversationTasks[conversationID] == taskID {
		delete(ctrl.conversationTasks, conversationID)
	}
}

// cancelConversationTask 取消会话的当前任务（同一会话串行处理）
func (ctrl *Controller) cancelConversationTask(conversationID uint) {
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	if taskID, ok := ctrl.conversationTasks[conversationID]; ok {
		if task, ok := ctrl.activeTasks[taskID]; ok {
			ctrl.logger.Info("取消会话的当前任务",
				zap.String("task_id", taskID),
				zap.Uint("conversation_id", conversationID),
				zap.String("reason", "新消息到达"),
			)
			task.Cancel()
			delete(ctrl.activeTasks, taskID)
			delete(ctrl.conversationTasks, conversationID)
		}
	}
}

// checkUserTaskLimit 检查用户并发任务数（O(1) 复杂度）
func (ctrl *Controller) checkUserTaskLimit(userID uint) error {
	ctrl.mu.RLock()
	count := ctrl.userTaskCount[userID]
	ctrl.mu.RUnlock()

	if count >= MaxConcurrentTasksPerUser {
		return fmt.Errorf("用户 %d 已达到最大并发任务数 %d", userID, MaxConcurrentTasksPerUser)
	}
	return nil
}

// getLLMClient 获取 LLM 客户端
func (ctrl *Controller) getLLMClient(agentSlug, modelName string) (llm.Client, error) {
	// TODO: 根据 agentSlug 或 modelName 获取对应的 LLM 配置
	// 临时实现：使用默认配置
	provider := "openai"
	apiKey := ""  // 从配置中获取
	baseURL := "" // 从配置中获取

	return ctrl.llmFactory(context.Background(), provider, modelName, apiKey, baseURL, 60)
}

// getMaxFileSize 根据文件类型获取最大文件大小
func (ctrl *Controller) getMaxFileSize(fileType string) int64 {
	switch fileType {
	case FileTypeImage:
		return MaxFileSizeImage
	case FileTypeDocument:
		return MaxFileSizeDocument
	case FileTypeCode:
		return MaxFileSizeCode
	case FileTypeAudio:
		return MaxFileSizeAudio
	case FileTypeVideo:
		return MaxFileSizeVideo
	default:
		return MaxFileSizeDocument // 默认20MB
	}
}

// UploadFile 上传文件
// POST /api/v1/chat/upload
func (ctrl *Controller) UploadFile(c *gin.Context) {
	var req FileUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(GetHTTPStatus(ErrCodeInvalidRequest), "请求参数无效: "+err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	conversationID := req.ConversationID

	// 1. 校验会话权限
	// TODO: 调用 ConversationService 校验会话属于当前用户

	// 2. 校验文件大小
	maxSize := ctrl.getMaxFileSize(req.FileType)
	if req.FileSize > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, NewErrorResponse(GetHTTPStatus(ErrCodeFileTooLarge),
			fmt.Sprintf("文件大小超过限制，最大允许 %dMB", maxSize/1024/1024)))
		return
	}

	// 3. 创建文件记录
	fileID := uint(0) // TODO: 从文件服务获取

	// 4. 生成预签名上传URL
	uploadURL := "" // TODO: 从 MinIO 生成预签名URL

	// 5. 记录日志
	ctrl.logger.Info("文件上传请求",
		zap.Uint("user_id", userID),
		zap.Uint("conversation_id", conversationID),
		zap.Uint("file_id", fileID),
		zap.String("file_name", req.FileName),
		zap.String("file_type", req.FileType),
		zap.Int64("file_size", req.FileSize),
	)

	c.JSON(http.StatusOK, NewSuccessResponse(FileUploadResponse{
		FileID:    fileID,
		FileName:  req.FileName,
		FileSize:  req.FileSize,
		FileType:  req.FileType,
		UploadURL: uploadURL,
	}))
}

// ProcessFile 处理已上传的文件
// POST /api/v1/chat/process-file
func (ctrl *Controller) ProcessFile(c *gin.Context) {
	var req struct {
		FileID uint `json:"file_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(GetHTTPStatus(ErrCodeInvalidRequest), "请求参数无效: "+err.Error()))
		return
	}

	userID := middleware.GetUserID(c)

	// 1. 获取文件信息
	// TODO: 从文件服务获取文件信息

	// 2. 处理文件
	// TODO: 根据文件类型进行处理

	// 3. 记录日志
	ctrl.logger.Info("文件处理请求",
		zap.Uint("user_id", userID),
		zap.Uint("file_id", req.FileID),
	)

	c.JSON(http.StatusOK, NewSuccessResponse(map[string]string{
		"message": "文件处理请求已接受",
	}))
}
