package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	contextmgr "Qavor/internal/context"
	"Qavor/internal/llm"
	"Qavor/internal/sse"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SSEServiceImpl SSE 服务实现
type SSEServiceImpl struct {
	contextMgr contextmgr.ContextManager
	llmFactory llm.ClientFactory
	config     *sse.SSEConfig
	logger     *zap.Logger
	heartbeat  *sse.HeartbeatManager

	// 任务管理：支持取消
	mu          sync.RWMutex
	activeTasks map[string]*SSETaskInfo // taskID -> TaskInfo

	// 会话任务管理：同一会话串行处理
	conversationTasks map[uint]string // conversationID -> taskID

	// 用户任务计数：优化并发检查 O(1) 复杂度
	userTaskCount map[uint]int // userID -> 当前任务数
}

// SSETaskInfo 任务信息
type SSETaskInfo struct {
	Cancel         context.CancelFunc
	UserID         uint
	TaskID         string
	ConversationID uint
}

// NewSSEService 创建 SSE 服务
func NewSSEService(
	contextMgr contextmgr.ContextManager,
	llmFactory llm.ClientFactory,
	config *sse.SSEConfig,
	logger *zap.Logger,
) *SSEServiceImpl {
	return &SSEServiceImpl{
		contextMgr:        contextMgr,
		llmFactory:        llmFactory,
		config:            config,
		logger:            logger,
		heartbeat:         sse.NewHeartbeatManager(config.HeartbeatInterval, logger),
		activeTasks:       make(map[string]*SSETaskInfo),
		conversationTasks: make(map[uint]string),
		userTaskCount:     make(map[uint]int),
	}
}

// Stream 处理流式对话
func (s *SSEServiceImpl) Stream(ctx context.Context, req *StreamRequest) error {
	taskID := req.TaskID
	userID := req.UserID
	conversationID := req.ConversationID
	writer := req.Writer

	// 1. 检查用户并发任务限制
	if err := s.checkUserTaskLimit(userID); err != nil {
		return err
	}

	// 2. 取消会话的当前任务（同一会话串行处理）
	s.cancelConversationTask(conversationID)

	// 3. 创建任务上下文（支持取消和超时）
	taskCtx, cancel := context.WithTimeout(ctx, s.config.MaxStreamTime)
	defer cancel()

	// 注册任务（支持外部取消）
	s.registerTask(taskID, cancel, userID, conversationID)
	defer s.unregisterTask(taskID, conversationID)

	// 记录日志：请求接收
	s.logger.Info("收到流式请求",
		zap.String("task_id", taskID),
		zap.Uint("user_id", userID),
		zap.Uint("conversation_id", conversationID),
		zap.String("model", req.ModelName),
	)

	// 4. 构建上下文
	query := &contextmgr.ContextHistoryQuery{
		ConversationID: conversationID,
		Limit:          50,
	}
	window, err := s.contextMgr.FetchContext(ctx, query)
	if err != nil {
		s.logger.Error("构建上下文失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		writer.Send(sse.EventMessageError, sse.ErrorData{
			Code:    "CONTEXT_BUILD_FAILED",
			Message: "构建上下文失败",
		})
		writer.Send(sse.EventDone, nil)
		return err
	}

	// 记录日志：上下文构建完成
	s.logger.Info("上下文构建完成",
		zap.String("task_id", taskID),
		zap.Int("history_count", len(window.Messages)),
	)

	// 5. 创建用户消息并持久化
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: req.Content,
	}
	userMsgID, err := s.contextMgr.PersistUserMessage(ctx, conversationID, userMessage)
	if err != nil {
		s.logger.Error("保存用户消息失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		// 不阻塞流程，继续执行
	}
	_ = userMsgID

	// 6. 组装 Prompt
	messages := s.contextMgr.BuildPrompt(ctx, window, userMessage)

	// 7. 获取 LLM 客户端
	llmClient, err := s.getLLMClient(req.AgentSlug, req.ModelName)
	if err != nil {
		s.logger.Error("LLM 初始化失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		writer.Send(sse.EventMessageError, sse.ErrorData{
			Code:    "LLM_INIT_FAILED",
			Message: "初始化LLM失败: " + err.Error(),
		})
		writer.Send(sse.EventDone, nil)
		return err
	}

	// 8. 生成消息ID
	messageID := fmt.Sprintf("msg_%s", uuid.New().String()[:8])

	// 9. 启动心跳
	stopHeartbeat := s.heartbeat.Start(taskCtx, writer, messageID)
	defer s.heartbeat.Stop(stopHeartbeat)

	// 10. 发送 message.start 事件
	writer.Send(sse.EventMessageStart, sse.MessageStartData{
		MessageID:      messageID,
		ConversationID: conversationID,
		Model:          req.ModelName,
	})

	// 记录日志：LLM 调用开始
	s.logger.Info("LLM 流式调用开始",
		zap.String("task_id", taskID),
		zap.String("message_id", messageID),
		zap.String("model", req.ModelName),
	)

	// 11. 调用 LLM 流式接口
	streamReader, err := llmClient.Stream(taskCtx, messages)
	if err != nil {
		s.logger.Error("LLM 流式调用失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		writer.Send(sse.EventMessageError, sse.ErrorData{
			Code:    "LLM_STREAM_FAILED",
			Message: "LLM调用失败: " + err.Error(),
		})
		writer.Send(sse.EventDone, nil)
		return err
	}

	// 12. 读取流式输出并推送事件
	var fullContent string
	var index int
	startTime := time.Now()

	for {
		chunk, err := streamReader.Recv()
		if err != nil {
			// 检查是否是正常结束
			if err.Error() == "EOF" {
				break
			}
			// 检查是否是取消
			if taskCtx.Err() == context.Canceled {
				s.logger.Info("任务已取消",
					zap.String("task_id", taskID),
					zap.String("message_id", messageID),
				)
				writer.Send(sse.EventMessageCancelled, sse.MessageCancelledData{
					MessageID: messageID,
					Reason:    "user_cancel",
				})
				writer.Send(sse.EventDone, nil)
				return nil
			}
			// 检查是否是超时
			if taskCtx.Err() == context.DeadlineExceeded {
				s.logger.Warn("任务超时",
					zap.String("task_id", taskID),
					zap.String("message_id", messageID),
				)
				writer.Send(sse.EventMessageError, sse.ErrorData{
					Code:    "TIMEOUT",
					Message: "请求超时",
				})
				writer.Send(sse.EventDone, nil)
				return fmt.Errorf("请求超时")
			}
			// 其他错误
			s.logger.Error("读取流式输出失败",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			writer.Send(sse.EventMessageError, sse.ErrorData{
				Code:    "STREAM_READ_FAILED",
				Message: "读取流式输出失败",
			})
			writer.Send(sse.EventDone, nil)
			return err
		}

		// 推送 message.delta 事件
		if chunk.Content != "" {
			fullContent += chunk.Content
			writer.Send(sse.EventMessageDelta, sse.MessageDeltaData{
				MessageID: messageID,
				Content:   chunk.Content,
				Index:     index,
			})
			index++
		}
	}

	// 13. 持久化 Assistant 消息
	assistantMessage := &schema.Message{
		Role:    schema.Assistant,
		Content: fullContent,
	}
	if err := s.contextMgr.PersistAssistantMessage(ctx, conversationID, assistantMessage); err != nil {
		s.logger.Error("保存Assistant消息失败",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
	}

	// 14. 发送 message.complete 事件
	duration := time.Since(startTime)
	writer.Send(sse.EventMessageComplete, sse.MessageCompleteData{
		MessageID:    messageID,
		Content:      fullContent,
		TokenCount:   0, // TODO: 从LLM响应中获取
		FinishReason: "stop",
	})

	// 记录日志：流式完成
	s.logger.Info("流式输出完成",
		zap.String("task_id", taskID),
		zap.String("message_id", messageID),
		zap.Duration("duration", duration),
	)

	// 15. 发送 done 事件
	writer.Send(sse.EventDone, nil)

	return nil
}

// Cancel 取消任务
func (s *SSEServiceImpl) Cancel(taskID string) error {
	s.mu.RLock()
	taskInfo, ok := s.activeTasks[taskID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在或已完成")
	}

	// 记录日志
	s.logger.Info("用户取消任务",
		zap.String("task_id", taskID),
		zap.Uint("user_id", taskInfo.UserID),
		zap.Uint("conversation_id", taskInfo.ConversationID),
	)

	// 取消任务
	taskInfo.Cancel()

	return nil
}

// UploadFile 上传文件
func (s *SSEServiceImpl) UploadFile(userID uint, req *FileUploadRequest) (*FileUploadResponse, error) {
	// 1. 校验会话权限
	// TODO: 调用 ConversationService 校验会话属于当前用户

	// 2. 校验文件大小
	maxSize := s.getMaxFileSize(req.FileType)
	if req.FileSize > maxSize {
		return nil, fmt.Errorf("文件大小超过限制，最大允许 %dMB", maxSize/1024/1024)
	}

	// 3. 创建文件记录
	fileID := uint(0) // TODO: 从文件服务获取

	// 4. 生成预签名上传URL
	uploadURL := "" // TODO: 从 MinIO 生成预签名URL

	// 5. 记录日志
	s.logger.Info("文件上传请求",
		zap.Uint("user_id", userID),
		zap.Uint("conversation_id", req.ConversationID),
		zap.Uint("file_id", fileID),
		zap.String("file_name", req.FileName),
		zap.String("file_type", req.FileType),
		zap.Int64("file_size", req.FileSize),
	)

	return &FileUploadResponse{
		FileID:    fileID,
		FileName:  req.FileName,
		FileSize:  req.FileSize,
		FileType:  req.FileType,
		UploadURL: uploadURL,
	}, nil
}

// ProcessFile 处理文件
func (s *SSEServiceImpl) ProcessFile(userID uint, fileID uint) error {
	// 1. 获取文件信息
	// TODO: 从文件服务获取文件信息

	// 2. 处理文件
	// TODO: 根据文件类型进行处理

	// 3. 记录日志
	s.logger.Info("文件处理请求",
		zap.Uint("user_id", userID),
		zap.Uint("file_id", fileID),
	)

	return nil
}

// registerTask 注册活跃任务
func (s *SSEServiceImpl) registerTask(taskID string, cancel context.CancelFunc, userID uint, conversationID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeTasks[taskID] = &SSETaskInfo{
		Cancel:         cancel,
		UserID:         userID,
		TaskID:         taskID,
		ConversationID: conversationID,
	}
	// 记录会话的当前任务
	s.conversationTasks[conversationID] = taskID
	// 增加用户任务计数
	s.userTaskCount[userID]++
}

// unregisterTask 注销活跃任务
func (s *SSEServiceImpl) unregisterTask(taskID string, conversationID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, ok := s.activeTasks[taskID]; ok {
		// 减少用户任务计数
		s.userTaskCount[task.UserID]--
		if s.userTaskCount[task.UserID] <= 0 {
			delete(s.userTaskCount, task.UserID)
		}
	}

	delete(s.activeTasks, taskID)

	// 如果当前会话的任务就是这个任务，清除映射
	if s.conversationTasks[conversationID] == taskID {
		delete(s.conversationTasks, conversationID)
	}
}

// cancelConversationTask 取消会话的当前任务（同一会话串行处理）
func (s *SSEServiceImpl) cancelConversationTask(conversationID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if taskID, ok := s.conversationTasks[conversationID]; ok {
		if task, ok := s.activeTasks[taskID]; ok {
			s.logger.Info("取消会话的当前任务",
				zap.String("task_id", taskID),
				zap.Uint("conversation_id", conversationID),
				zap.String("reason", "新消息到达"),
			)
			task.Cancel()
			delete(s.activeTasks, taskID)
			delete(s.conversationTasks, conversationID)
		}
	}
}

// checkUserTaskLimit 检查用户并发任务数（O(1) 复杂度）
func (s *SSEServiceImpl) checkUserTaskLimit(userID uint) error {
	s.mu.RLock()
	count := s.userTaskCount[userID]
	s.mu.RUnlock()

	if count >= s.config.MaxConcurrentTasks {
		return fmt.Errorf("用户 %d 已达到最大并发任务数 %d", userID, s.config.MaxConcurrentTasks)
	}
	return nil
}

// getLLMClient 获取 LLM 客户端
func (s *SSEServiceImpl) getLLMClient(agentSlug, modelName string) (llm.Client, error) {
	// TODO: 根据 agentSlug 或 modelName 获取对应的 LLM 配置
	// 临时实现：使用默认配置
	provider := "openai"
	apiKey := ""  // 从配置中获取
	baseURL := "" // 从配置中获取

	return s.llmFactory(context.Background(), provider, modelName, apiKey, baseURL, 60)
}

// getMaxFileSize 根据文件类型获取最大文件大小
func (s *SSEServiceImpl) getMaxFileSize(fileType string) int64 {
	switch fileType {
	case "image":
		return 10 * 1024 * 1024 // 10MB
	case "document":
		return 20 * 1024 * 1024 // 20MB
	case "code":
		return 1 * 1024 * 1024 // 1MB
	case "audio":
		return 50 * 1024 * 1024 // 50MB
	case "video":
		return 100 * 1024 * 1024 // 100MB
	default:
		return 20 * 1024 * 1024 // 默认20MB
	}
}
