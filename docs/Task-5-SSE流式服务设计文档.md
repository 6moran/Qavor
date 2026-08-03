# Task 5: SSE 流式服务设计文档

## 文档状态
- **状态**: ✅ 已实现
- **完成时间**: 2026-08-01
- **相关提交**: b9c0816, b284a87, 4a79093, 8528424, 3e5edc3, 3267666, 587f7a2, c108d87, 203496b, 9ce5e5a

## 文档信息
- **项目**：Qavor Agent 对话系统
- **模块**：SSE 流式服务（Server-Sent Events Streaming）
- **依赖模块**：上下文管理（Task 4）、LLM 抽象层、文件服务
- **下游模块**：前端聊天 UI
- **目标**：搭建 SSE 流式服务，支持任务级连接、文件上传、用户操作事件

---

# 1 概述

SSE 流式服务负责将 LLM 的流式输出实时推送给前端客户端。

**连接模型**：任务级连接（一个消息一个连接）

**关键特性**：同一会话串行处理

```
客户端发送消息 → POST /chat/stream
    │
    ├── 取消会话的当前任务（同一会话串行处理）
    │
    ├── 服务端设置 SSE 响应头
    │
    ├── 构建上下文（历史消息 + 用户消息）
    │
    ├── 调用 LLM 流式接口
    │
    ├── 推送 SSE 事件（message.start → message.delta × N → message.complete）
    │
    └── 完成后关闭连接
```

**核心优势**：
- **同一会话串行处理**：新消息到达时，自动取消旧消息的处理
- **文件上传支持**：支持图片、文档、代码等多种文件类型
- **用户操作事件**：支持中断、重新生成、编辑消息
- 生命周期清晰：请求开始 → 流式输出 → 请求结束
- 资源友好：无需维护长连接状态
- 实现简单：无需空闲清理、断线重连

---

# 2 SSE 响应头说明

SSE 连接需要设置特殊的 HTTP 响应头，确保浏览器和代理服务器正确处理流式响应：

```go
// 必须设置的响应头
c.Header("Content-Type", "text/event-stream")  // 指定 SSE 协议
c.Header("Cache-Control", "no-cache")          // 禁用缓存
c.Header("Connection", "keep-alive")            // 保持连接
c.Header("X-Accel-Buffering", "no")            // 禁用 Nginx 缓冲
```

| Header | 值 | 作用 |
|--------|-----|------|
| `Content-Type` | `text/event-stream` | 告诉浏览器这是 SSE 流 |
| `Cache-Control` | `no-cache` | 禁用缓存，确保实时推送 |
| `Connection` | `keep-alive` | 保持 TCP 连接 |
| `X-Accel-Buffering` | `no` | 禁用 Nginx 代理缓冲，避免延迟 |

**可选 Header**（根据部署环境）：

```go
// 如果使用反向代理，可能需要设置
c.Header("X-Accel-Redirect", "/dev/null")  // 某些代理需要
```

---

# 3 目录结构

```
internal/sse/
├── controller.go       // SSE HTTP Handler
├── writer.go           // SSE 线程安全写入器
├── events.go           // 事件类型定义
├── types.go            // SSE 相关类型
├── config.go           // SSE 配置
├── heartbeat.go        // 心跳管理
└── errors.go           // 错误码定义

internal/api/v1/chat/
├── routes.go           // 路由定义
└── controller.go       // 聊天控制器（调用 SSE Controller）

internal/file/
├── service.go          // 文件服务接口
├── minio.go            // MinIO 实现
└── processor.go        // 文件处理器（OCR、解析等）
```

---

# 3 类型定义 (types.go)

```go
package sse

import "time"

const (
    // MaxConcurrentTasksPerUser 单用户最大并发任务数
    MaxConcurrentTasksPerUser = 5
)

// SSEConfig SSE 配置
type SSEConfig struct {
    MaxStreamTime     time.Duration // 单次流式最大时长
    HeartbeatInterval time.Duration // 流式过程中心跳间隔
}

// StreamRequest 流式对话请求
type StreamRequest struct {
    ConversationID uint   `json:"conversation_id" binding:"required"`
    Content        string `json:"content" binding:"required"`
    AgentSlug      string `json:"agent_slug"`       // 可选：指定 Agent
    ModelName      string `json:"model_name"`       // 可选：指定模型
    FileIDs        []uint `json:"file_ids"`         // 可选：关联的文件ID列表
}

// FileUploadRequest 文件上传请求
type FileUploadRequest struct {
    ConversationID uint   `json:"conversation_id" binding:"required"`
    FileType       string `json:"file_type" binding:"required,oneof=image document code audio video"`
    FileName       string `json:"file_name" binding:"required"`
    FileSize       int64  `json:"file_size" binding:"required"`
}

// 支持的文件类型
const (
    FileTypeImage    = "image"    // 图片：jpg, png, gif, webp
    FileTypeDocument = "document" // 文档：pdf, doc, docx, txt, md
    FileTypeCode     = "code"     // 代码：js, ts, py, go, java, etc.
    FileTypeAudio    = "audio"    // 音频：mp3, wav, m4a
    FileTypeVideo    = "video"    // 视频：mp4, webm, avi
)

// 文件大小限制
const (
    MaxFileSizeImage    = 10 * 1024 * 1024  // 10MB
    MaxFileSizeDocument = 20 * 1024 * 1024  // 20MB
    MaxFileSizeCode     = 1 * 1024 * 1024   // 1MB
    MaxFileSizeAudio    = 50 * 1024 * 1024  // 50MB
    MaxFileSizeVideo    = 100 * 1024 * 1024 // 100MB
)

// FileUploadResponse 文件上传响应
type FileUploadResponse struct {
    FileID    uint   `json:"file_id"`
    FileName  string `json:"file_name"`
    FileSize  int64  `json:"file_size"`
    FileType  string `json:"file_type"`
    UploadURL string `json:"upload_url"` // 预签名上传URL
}

// CancelRequest 取消请求
type CancelRequest struct {
    TaskID string `json:"task_id" binding:"required"`
}

// RegenerateRequest 重新生成请求
type RegenerateRequest struct {
    ConversationID uint `json:"conversation_id" binding:"required"`
}

// EditRequest 编辑消息请求
type EditRequest struct {
    ConversationID uint   `json:"conversation_id" binding:"required"`
    MessageID      uint   `json:"message_id" binding:"required"`      // 原始消息ID
    NewContent     string `json:"new_content" binding:"required"`     // 新的消息内容
}

// StreamResponse 流式对话响应（非SSE，用于错误场景）
type StreamResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
}
```

---

# 4 事件协议定义 (events.go)

## 4.1 事件格式

SSE 标准格式：
```
event: <event_type>
data: <json_payload>

```

## 4.2 统一响应结构

与项目现有DTO保持一致，使用统一的响应包装：

```go
package sse

// Response 统一响应结构（与 internal/model/dto/response/common.go 一致）
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse 成功响应
func SuccessResponse(data interface{}) *Response {
    return &Response{
        Code:    200,
        Message: "success",
        Data:    data,
    }
}

// ErrorResponse 错误响应
func ErrorResponse(code int, message string) *Response {
    return &Response{
        Code:    code,
        Message: message,
    }
}
```

## 4.3 事件类型

```go
// EventType SSE 事件类型
type EventType string

const (
    // 消息事件
    EventMessageStart    EventType = "message.start"    // 消息开始
    EventMessageDelta    EventType = "message.delta"    // 增量内容
    EventMessageComplete EventType = "message.complete" // 消息完成
    EventMessageError    EventType = "message.error"    // 消息错误
    EventMessageCancelled EventType = "message.cancelled" // 消息取消（用户中断）
    EventMessageRegenerate EventType = "message.regenerate" // 消息重新生成
    EventMessageEdited    EventType = "message.edited"     // 消息已编辑

    // 文件事件
    EventFileUploadStart    EventType = "file.upload_start"    // 文件上传开始
    EventFileUploadProgress EventType = "file.upload_progress" // 文件上传进度
    EventFileUploadComplete EventType = "file.upload_complete" // 文件上传完成
    EventFileUploadError    EventType = "file.upload_error"    // 文件上传失败
    EventFileProcessStart   EventType = "file.process_start"   // 文件处理开始
    EventFileProcessDone    EventType = "file.process_done"    // 文件处理完成
    EventFileProcessError   EventType = "file.process_error"   // 文件处理失败

    // 工具调用事件（Agent调用MCP工具时）
    EventToolCallStart   EventType = "tool_call.start"  // 工具调用开始
    EventToolCallEnd     EventType = "tool_call.end"    // 工具调用结束

    // RAG 事件（知识库检索时）
    EventRAGStart        EventType = "rag.start"        // 检索开始
    EventRAGDone         EventType = "rag.done"         // 检索完成

    // 心跳事件
    EventHeartbeat       EventType = "heartbeat"        // 心跳

    // 流结束
    EventDone            EventType = "done"             // 流结束
)
```

## 4.4 用户操作事件

### 4.4.1 中断（Cancel）

**触发条件**：用户点击"停止生成"按钮

**后端处理**：
```
用户点击停止 → POST /api/v1/chat/cancel/:task_id
    │
    ├── 取消 context（中断 LLM 生成）
    │
    ├── 推送 message.cancelled 事件
    │
    └── 关闭 SSE 连接
```

**前端行为**：
- 显示"已中断"提示
- 保留已生成的部分内容
- 允许用户重新发送或编辑消息

### 4.4.2 重新生成（Regenerate）

**触发条件**：用户点击"重新生成"按钮

**后端处理**：
```
用户点击重新生成 → POST /api/v1/chat/regenerate
    │
    ├── 获取当前会话的最后一条用户消息
    │
    ├── 删除当前会话的最后一条 Assistant 消息
    │
    ├── 重新构建上下文（不包含已删除的 Assistant 消息）
    │
    ├── 调用 LLM 流式接口
    │
    └── 推送新的消息流
```

**前端行为**：
- 清除当前 Assistant 消息的显示
- 显示"重新生成中..."提示
- 接收新的消息流并显示

### 4.4.3 用户修改了发送的消息（Edit）

**触发条件**：用户编辑已发送的消息并重新发送

**后端处理**：
```
用户编辑消息并重新发送 → POST /api/v1/chat/edit
    │
    ├── 获取原始消息ID
    │
    ├── 更新原始消息内容
    │
    ├── 删除该消息之后的所有 Assistant 消息
    │
    ├── 重新构建上下文
    │
    ├── 调用 LLM 流式接口
    │
    └── 推送新的消息流
```

**前端行为**：
- 更新原始消息的显示
- 清除该消息之后的所有 Assistant 消息
- 显示"处理中..."提示
- 接收新的消息流并显示

## 4.4 错误码定义

```go
package sse

// 错误码常量
const (
    // 客户端错误 (4xx)
    ErrCodeInvalidRequest      = "INVALID_REQUEST"       // 请求参数无效
    ErrCodeUnauthorized        = "UNAUTHORIZED"           // 未授权
    ErrCodeForbidden           = "FORBIDDEN"              // 无权限
    ErrCodeNotFound            = "NOT_FOUND"              // 资源不存在
    ErrCodeTooManyRequests     = "TOO_MANY_REQUESTS"      // 请求过多

    // 服务端错误 (5xx)
    ErrCodeInternalError       = "INTERNAL_ERROR"         // 内部错误
    ErrCodeContextBuildFailed  = "CONTEXT_BUILD_FAILED"   // 上下文构建失败
    ErrCodeLLMInitFailed       = "LLM_INIT_FAILED"        // LLM初始化失败
    ErrCodeLLMStreamFailed     = "LLM_STREAM_FAILED"      // LLM流式调用失败
    ErrCodeStreamReadFailed    = "STREAM_READ_FAILED"     // 流式读取失败
    ErrCodePersistFailed       = "PERSIST_FAILED"         // 持久化失败
    ErrCodeTaskNotFound        = "TASK_NOT_FOUND"         // 任务不存在
    ErrCodeTaskCancelled       = "TASK_CANCELLED"         // 任务已取消
    ErrCodeTimeout             = "TIMEOUT"                // 超时

    // 文件错误
    ErrCodeFileTooLarge        = "FILE_TOO_LARGE"         // 文件大小超限
    ErrCodeUnsupportedFileType = "UNSUPPORTED_FILE_TYPE"  // 不支持的文件类型
    ErrCodeFileParseFailed     = "FILE_PARSE_FAILED"      // 文件解析失败
    ErrCodeOCRFailed           = "OCR_FAILED"             // OCR识别失败
    ErrCodeStorageUnavailable  = "STORAGE_UNAVAILABLE"    // 存储服务不可用
    ErrCodeFileProcessFailed   = "FILE_PROCESS_FAILED"    // 文件处理失败

    // 业务错误
    ErrCodeConversationNotFound    = "CONVERSATION_NOT_FOUND"    // 会话不存在
    ErrCodeConversationClosed      = "CONVERSATION_CLOSED"       // 会话已关闭
    ErrCodeMessageNotFound         = "MESSAGE_NOT_FOUND"         // 消息不存在
)

// 错误码与HTTP状态码映射
var errorCodeToHTTPStatus = map[string]int{
    ErrCodeInvalidRequest:      400,
    ErrCodeUnauthorized:        401,
    ErrCodeForbidden:           403,
    ErrCodeNotFound:            404,
    ErrCodeTooManyRequests:     429,
    ErrCodeInternalError:       500,
    ErrCodeContextBuildFailed:  500,
    ErrCodeLLMInitFailed:       500,
    ErrCodeLLMStreamFailed:     500,
    ErrCodeStreamReadFailed:    500,
    ErrCodePersistFailed:       500,
    ErrCodeTaskNotFound:        404,
    ErrCodeTaskCancelled:       499,  // 自定义状态码
    ErrCodeTimeout:             504,
    ErrCodeFileTooLarge:        413,  // Payload Too Large
    ErrCodeUnsupportedFileType: 415,  // Unsupported Media Type
    ErrCodeFileParseFailed:     422,  // Unprocessable Entity
    ErrCodeOCRFailed:           422,
    ErrCodeStorageUnavailable:  503,  // Service Unavailable
    ErrCodeFileProcessFailed:   422,
    ErrCodeConversationNotFound: 404,
    ErrCodeConversationClosed:  400,
    ErrCodeMessageNotFound:     404,
}

// GetHTTPStatus 获取错误码对应的HTTP状态码
func GetHTTPStatus(errCode string) int {
    if status, ok := errorCodeToHTTPStatus[errCode]; ok {
        return status
    }
    return 500
}
```

## 4.4 事件数据结构

```go
// SSEEvent SSE 事件（统一格式）
type SSEEvent struct {
    Type EventType   `json:"type"`
    Data interface{} `json:"data"`
}

// --- 消息事件数据 ---

// MessageStartData 消息开始事件
type MessageStartData struct {
    MessageID      string `json:"message_id"`
    ConversationID uint   `json:"conversation_id"`
    Model          string `json:"model"`
}

// MessageDeltaData 增量内容事件
type MessageDeltaData struct {
    MessageID string `json:"message_id"`
    Content   string `json:"content"`   // 增量内容片段
    Index     int    `json:"index"`     // 片段序号
}

// MessageCompleteData 消息完成事件
type MessageCompleteData struct {
    MessageID    string `json:"message_id"`
    Content      string `json:"content"`       // 完整内容
    TokenCount   int    `json:"token_count"`   // Token 消耗
    FinishReason string `json:"finish_reason"` // 结束原因
}

// MessageCancelledData 消息取消事件
type MessageCancelledData struct {
    MessageID string `json:"message_id"`
    Reason    string `json:"reason"`    // 取消原因：user_cancel / new_message / regenerate / edit
}

// MessageRegenerateData 消息重新生成事件
type MessageRegenerateData struct {
    OriginalMessageID string `json:"original_message_id"` // 原始消息ID
    ConversationID    uint   `json:"conversation_id"`
}

// MessageEditedData 消息已编辑事件
type MessageEditedData struct {
    OriginalMessageID string `json:"original_message_id"` // 原始消息ID
    NewContent        string `json:"new_content"`         // 新的消息内容
    DeletedMessages   int    `json:"deleted_messages"`    // 删除的消息数量
}

// --- 文件事件数据 ---

// FileUploadStartData 文件上传开始事件
type FileUploadStartData struct {
    FileID   uint   `json:"file_id"`
    FileName string `json:"file_name"`
    FileSize int64  `json:"file_size"`
    FileType string `json:"file_type"`
}

// FileUploadProgressData 文件上传进度事件
type FileUploadProgressData struct {
    FileID   uint    `json:"file_id"`
    Progress float64 `json:"progress"` // 0.0 - 1.0
    Loaded   int64   `json:"loaded"`
    Total    int64   `json:"total"`
}

// FileUploadCompleteData 文件上传完成事件
type FileUploadCompleteData struct {
    FileID    uint   `json:"file_id"`
    FileName  string `json:"file_name"`
    FileSize  int64  `json:"file_size"`
    FileType  string `json:"file_type"`
    FileURL   string `json:"file_url"`   // 文件访问URL
    PreviewURL string `json:"preview_url"` // 预览URL（图片等）
}

// FileProcessStartData 文件处理开始事件
type FileProcessStartData struct {
    FileID   uint   `json:"file_id"`
    FileName string `json:"file_name"`
    Process  string `json:"process"` // 处理类型：ocr / parse / extract
}

// FileProcessDoneData 文件处理完成事件
type FileProcessDoneData struct {
    FileID       uint   `json:"file_id"`
    FileName     string `json:"file_name"`
    Content      string `json:"content"`       // 提取的文本内容
    TokenCount   int    `json:"token_count"`   // 内容的Token数
    ProcessTime  int64  `json:"process_time"`  // 处理耗时（ms）
    Metadata     map[string]interface{} `json:"metadata,omitempty"` // 额外元数据
}

// FileProcessErrorData 文件处理失败事件
type FileProcessErrorData struct {
    FileID   uint   `json:"file_id"`
    FileName string `json:"file_name"`
    Error    string `json:"error"`
    Code     string `json:"code"` // 错误码
}

// --- 文件处理方式 ---

// 不同文件类型的处理方式
// | 文件类型 | 处理方式 | 说明 |
// |----------|----------|------|
// | image | OCR / 视觉模型 | 提取图片中的文字或描述 |
// | document | 文本解析 | 提取文档内容 |
// | code | 语法分析 | 提取代码结构和注释 |
// | audio | 语音识别 | 转换为文本 |
// | video | 视频解析 | 提取关键帧和字幕 |

// --- 工具调用事件数据 ---

// ToolCallStartData 工具调用开始事件
type ToolCallStartData struct {
    MessageID  string `json:"message_id"`
    ToolName   string `json:"tool_name"`
    ToolCallID string `json:"tool_call_id"`
}

// ToolCallEndData 工具调用结束事件
type ToolCallEndData struct {
    MessageID  string `json:"message_id"`
    ToolName   string `json:"tool_name"`
    ToolCallID string `json:"tool_call_id"`
    Success    bool   `json:"success"`
}

// --- RAG 事件数据 ---

// RAGStartData RAG检索开始事件
type RAGStartData struct {
    Query           string `json:"query"`
    KnowledgeBaseID uint   `json:"knowledge_base_id"`
}

// RAGDoneData RAG检索完成事件
type RAGDoneData struct {
    Query        string `json:"query"`
    ResultCount  int    `json:"result_count"`
    ChunksUsed   int    `json:"chunks_used"`
}

// --- 心跳事件数据 ---

// HeartbeatData 心跳事件
type HeartbeatData struct {
    MessageID string `json:"message_id"`
    Timestamp int64  `json:"timestamp"`
}

// --- 错误事件数据 ---

// ErrorData 错误事件
type ErrorData struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

## 4.5 并发控制

### 任务ID生成策略

任务级连接下，每个请求生成唯一的任务ID，用于：
- 取消任务时标识目标
- 日志追踪
- 前端关联请求

```go
// 任务ID格式：task_{uuid前8位}_{时间戳后6位}
// 示例：task_a1b2c3d4_230801

func GenerateTaskID() string {
    uuid := uuid.New().String()[:8]
    timestamp := time.Now().Unix() % 1000000
    return fmt.Sprintf("task_%s_%06d", uuid, timestamp)
}
```

### 多实例部署考虑

在多实例部署时，任务取消需要考虑跨实例通信：

```
方案1：客户端轮询（推荐）
  - 前端发送取消请求到任意实例
  - 该实例尝试取消本地任务
  - 如果任务不在本地，返回 404
  - 前端可以尝试其他实例（如果知道）

方案2：Redis 分布式锁
  - 任务创建时在 Redis 记录实例ID
  - 取消时先查 Redis，路由到正确实例
  - 复杂度较高，适合大规模部署
```

**当前方案**：单实例部署，使用内存 map 存储任务取消函数。

### 并发限制

```go
// 单用户最大并发任务数
const MaxConcurrentTasksPerUser = 5

// 检查用户并发任务数
func (ctrl *Controller) checkUserTaskLimit(userID uint) error {
    ctrl.mu.RLock()
    defer ctrl.mu.RUnlock()

    count := 0
    for _, task := range ctrl.activeTasks {
        if task.UserID == userID {
            count++
        }
    }

    if count >= MaxConcurrentTasksPerUser {
        return fmt.Errorf("用户 %d 已达到最大并发任务数 %d", userID, MaxConcurrentTasksPerUser)
    }
    return nil
}
```

## 4.6 日志规范

### 关键节点日志

```go
// 1. 请求接收
ctrl.logger.Info("收到流式请求",
    zap.String("task_id", taskID),
    zap.Uint("user_id", userID),
    zap.Uint("conversation_id", conversationID),
    zap.String("model", req.ModelName),
)

// 2. 上下文构建
ctrl.logger.Info("上下文构建完成",
    zap.String("task_id", taskID),
    zap.Int("history_count", len(window.Messages)),
)

// 3. LLM 调用开始
ctrl.logger.Info("LLM 流式调用开始",
    zap.String("task_id", taskID),
    zap.String("model", req.ModelName),
)

// 4. 心跳发送（可选，避免日志过多）
ctrl.logger.Debug("心跳发送",
    zap.String("task_id", taskID),
    zap.String("message_id", messageID),
)

// 5. 流式完成
ctrl.logger.Info("流式输出完成",
    zap.String("task_id", taskID),
    zap.String("message_id", messageID),
    zap.Int("token_count", tokenCount),
    zap.Duration("duration", duration),
)

// 6. 错误发生
ctrl.logger.Error("流式处理失败",
    zap.String("task_id", taskID),
    zap.String("error_code", errCode),
    zap.Error(err),
)

// 7. 任务取消
ctrl.logger.Info("任务已取消",
    zap.String("task_id", taskID),
    zap.String("reason", reason),
)
```

### 日志字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `task_id` | string | 任务ID，用于追踪 |
| `user_id` | uint | 用户ID |
| `conversation_id` | uint | 会话ID |
| `message_id` | string | 消息ID |
| `model` | string | 使用的模型 |
| `token_count` | int | Token消耗 |
| `duration` | duration | 处理耗时 |
| `error_code` | string | 错误码 |

### 日志级别使用

| 级别 | 使用场景 |
|------|----------|
| `Debug` | 心跳发送、详细调试信息 |
| `Info` | 请求接收、处理完成、任务取消 |
| `Warn` | 非关键错误（如持久化失败但不影响主流程） |
| `Error` | 关键错误（如LLM调用失败） |

## 4.7 同一会话串行处理流程

```
用户在会话A发送消息1
    │
    ├── 建立SSE连接1
    │
    ├── 开始处理消息1
    │   ├── 推送: message.start
    │   ├── 推送: message.delta × 3
    │   │
    │   │   ← 用户在会话A发送消息2
    │   │
    │   ├── 检测到新消息到达
    │   ├── 取消消息1的处理
    │   ├── 推送: message.cancelled
    │   ├── 关闭SSE连接1
    │   │
    │   └── 建立SSE连接2
    │       ├── 推送: message.start
    │       ├── 推送: message.delta × N
    │       ├── 推送: message.complete
    │       └── 关闭SSE连接2
```

**关键行为**：
- 消息1被取消，用户收到 `message.cancelled` 事件
- 消息2开始处理，用户收到新的消息流
- 两个SSE连接是独立的，但会话级别的任务是串行的

## 4.8 事件流示例

**简单对话**：
```
event: message.start
data: {"type":"message.start","data":{"message_id":"msg_456","conversation_id":1,"model":"gpt-4o"}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_456","content":"你好","index":0}}

event: heartbeat
data: {"type":"heartbeat","data":{"message_id":"msg_456","timestamp":1690000015}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_456","content":"！有什么","index":1}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_456","content":"我可以帮助","index":2}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_456","content":"你的吗？","index":3}}

event: message.complete
data: {"type":"message.complete","data":{"message_id":"msg_456","content":"你好！有什么我可以帮助你的吗？","token_count":15,"finish_reason":"stop"}}

event: done
data: {"type":"done","data":null}
```

**带工具调用的对话**（包含心跳）：
```
event: message.start
data: {"type":"message.start","data":{"message_id":"msg_789","conversation_id":1,"model":"gpt-4o"}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_789","content":"让我帮你查询","index":0}}

event: tool_call.start
data: {"type":"tool_call.start","data":{"message_id":"msg_789","tool_name":"search_knowledge","tool_call_id":"tc_001"}}

event: heartbeat
data: {"type":"heartbeat","data":{"message_id":"msg_789","timestamp":1690000030}}

event: tool_call.end
data: {"type":"tool_call.end","data":{"message_id":"msg_789","tool_name":"search_knowledge","tool_call_id":"tc_001","success":true}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_789","content":"根据知识库，答案是...","index":1}}

event: message.complete
data: {"type":"message.complete","data":{"message_id":"msg_789","content":"根据知识库，答案是...","token_count":25,"finish_reason":"stop"}}

event: done
data: {"type":"done","data":null}
```

**错误场景**：
```
event: message.error
data: {"type":"message.error","data":{"code":"LLM_STREAM_FAILED","message":"LLM调用失败: 超时"}}

event: done
data: {"type":"done","data":null}
```

**取消场景（用户中断）**：
```
event: message.cancelled
data: {"type":"message.cancelled","data":{"message_id":"msg_789","reason":"user_cancel"}}

event: done
data: {"type":"done","data":null}
```

**取消场景（新消息到达）**：
```
event: message.cancelled
data: {"type":"message.cancelled","data":{"message_id":"msg_789","reason":"new_message"}}

event: done
data: {"type":"done","data":null}
```

**重新生成场景**：
```
// 用户点击重新生成按钮
// 后端处理：删除最后一条 Assistant 消息，重新生成

event: message.start
data: {"type":"message.start","data":{"message_id":"msg_012","conversation_id":1,"model":"gpt-4o"}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_012","content":"重新生成的回答...","index":0}}

event: message.complete
data: {"type":"message.complete","data":{"message_id":"msg_012","content":"重新生成的回答...","token_count":20,"finish_reason":"stop"}}

event: done
data: {"type":"done","data":null}
```

**编辑消息场景**：
```
// 用户编辑了消息内容并重新发送
// 后端处理：更新消息内容，删除后续消息，重新生成

event: message.start
data: {"type":"message.start","data":{"message_id":"msg_345","conversation_id":1,"model":"gpt-4o"}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_345","content":"编辑后的回答...","index":0}}

event: message.complete
data: {"type":"message.complete","data":{"message_id":"msg_345","content":"编辑后的回答...","token_count":18,"finish_reason":"stop"}}

event: done
data: {"type":"done","data":null}
```

**文件上传场景**：
```
// 用户上传图片并发送消息

event: file.upload_start
data: {"type":"file.upload_start","data":{"file_id":123,"file_name":"screenshot.png","file_size":1024000,"file_type":"image"}}

event: file.upload_progress
data: {"type":"file.upload_progress","data":{"file_id":123,"progress":0.5,"loaded":512000,"total":1024000}}

event: file.upload_progress
data: {"type":"file.upload_progress","data":{"file_id":123,"progress":1.0,"loaded":1024000,"total":1024000}}

event: file.upload_complete
data: {"type":"file.upload_complete","data":{"file_id":123,"file_name":"screenshot.png","file_size":1024000,"file_type":"image","file_url":"https://storage.example.com/files/123.png"}}

event: file.process_start
data: {"type":"file.process_start","data":{"file_id":123,"file_name":"screenshot.png","process":"ocr"}}

event: file.process_done
data: {"type":"file.process_done","data":{"file_id":123,"file_name":"screenshot.png","content":"图片中的文字内容...","token_count":50,"process_time":1500}}

event: message.start
data: {"type":"message.start","data":{"message_id":"msg_678","conversation_id":1,"model":"gpt-4o"}}

event: message.delta
data: {"type":"message.delta","data":{"message_id":"msg_678","content":"我看到了图片中的内容","index":0}}

event: message.complete
data: {"type":"message.complete","data":{"message_id":"msg_678","content":"我看到了图片中的内容...","token_count":30,"finish_reason":"stop"}}

event: done
data: {"type":"done","data":null}
```

**文件处理失败场景**：
```
event: file.upload_complete
data: {"type":"file.upload_complete","data":{"file_id":124,"file_name":"document.pdf","file_size":5242880,"file_type":"document","file_url":"https://storage.example.com/files/124.pdf"}}

event: file.process_start
data: {"type":"file.process_start","data":{"file_id":124,"file_name":"document.pdf","process":"parse"}}

event: file.process_error
data: {"type":"file.process_error","data":{"file_id":124,"file_name":"document.pdf","error":"PDF解析失败：文件损坏","code":"FILE_PARSE_FAILED"}}

event: message.error
data: {"type":"message.error","data":{"code":"FILE_PROCESS_FAILED","message":"文件处理失败: PDF解析失败"}}

event: done
data: {"type":"done","data":null}
```

---

# 5 SSE Controller (controller.go)

## 5.1 职责
- 处理 HTTP 请求，建立任务级 SSE 连接
- 调用上下文管理器构建 Prompt
- 调用 LLM 获取流式输出
- 通过事件分发器推送事件
- 支持任务取消

## 5.2 实现

```go
package sse

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "sync"

    contextmgr "Qavor/internal/context"
    "Qavor/internal/llm"
    "Qavor/internal/middleware"
    "Qavor/internal/repository"
    "Qavor/internal/service"

    "github.com/gin-gonic/gin"
    "github.com/cloudwego/eino/schema"
    "github.com/google/uuid"
    "go.uber.org/zap"
)

// Controller SSE 控制器
type Controller struct {
    contextMgr  contextmgr.Manager
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
    contextMgr contextmgr.Manager,
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

// Stream 处理 SSE 流式请求
// POST /api/v1/chat/stream
func (ctrl *Controller) Stream(c *gin.Context) {
    // 1. 解析请求
    var req StreamRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidRequest, "请求参数无效: "+err.Error()))
        return
    }

    userID := middleware.GetUserID(c)
    conversationID := req.ConversationID
    taskID := GenerateTaskID()

    // 2. 检查用户并发任务限制
    if err := ctrl.checkUserTaskLimit(userID); err != nil {
        c.JSON(http.StatusTooManyRequests, ErrorResponse(ErrCodeTooManyRequests, err.Error()))
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
    query := &contextmgr.HistoryQuery{
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

    // 10. 处理文件内容（如果有）
    var fileContents []string
    if len(req.FileIDs) > 0 {
        for _, fileID := range req.FileIDs {
            // TODO: 从文件服务获取文件内容
            // fileContent, err := ctrl.fileService.GetFileContent(fileID)
            // if err != nil {
            //     ctrl.logger.Error("获取文件内容失败", zap.Uint("file_id", fileID), zap.Error(err))
            //     continue
            // }
            // fileContents = append(fileContents, fileContent)
        }
    }

    // 11. 组装 Prompt（包含文件内容）
    messages := ctrl.contextMgr.BuildPrompt(ctx, window, userMessage)

    // 12. 获取 LLM 客户端
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

    // 13. 生成消息ID
    messageID := fmt.Sprintf("msg_%s", uuid.New().String()[:8])

    // 14. 启动心跳
    stopHeartbeat := ctrl.heartbeat.Start(ctx, writer, messageID)
    defer ctrl.heartbeat.Stop(stopHeartbeat)

    // 15. 发送 message.start 事件
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

    // 16. 调用 LLM 流式接口
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

    // 17. 读取流式输出并推送事件
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
        if chunk.Content != "" {
            fullContent += chunk.Content
            writer.Send(EventMessageDelta, MessageDeltaData{
                MessageID: messageID,
                Content:   chunk.Content,
                Index:     index,
            })
            index++
        }
    }

    // 18. 持久化 Assistant 消息
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

    // 19. 发送 message.complete 事件
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
        zap.Int("token_count", 0),
        zap.Duration("duration", duration),
    )

    // 20. 发送 done 事件
    writer.Send(EventDone, nil)
}

// Cancel 取消正在生成的消息
// POST /api/v1/chat/cancel
func (ctrl *Controller) Cancel(c *gin.Context) {
    var req CancelRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidRequest, "请求参数无效: "+err.Error()))
        return
    }

    ctrl.mu.RLock()
    taskInfo, ok := ctrl.activeTasks[req.TaskID]
    ctrl.mu.RUnlock()

    if !ok {
        c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeTaskNotFound, "任务不存在或已完成"))
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

    c.JSON(http.StatusOK, SuccessResponse(map[string]string{"message": "任务已取消"}))
}

// Regenerate 重新生成消息（SSE 接口）
// POST /api/v1/chat/regenerate
func (ctrl *Controller) Regenerate(c *gin.Context) {
    var req RegenerateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidRequest, "请求参数无效: "+err.Error()))
        return
    }

    userID := middleware.GetUserID(c)
    conversationID := req.ConversationID
    taskID := GenerateTaskID()

    // 1. 检查用户并发任务限制
    if err := ctrl.checkUserTaskLimit(userID); err != nil {
        c.JSON(http.StatusTooManyRequests, ErrorResponse(ErrCodeTooManyRequests, err.Error()))
        return
    }

    // 2. 取消会话的当前任务
    ctrl.cancelConversationTask(conversationID)

    // 3. 设置 SSE 响应头
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")

    // 4. 创建 SSE 写入器
    writer := NewSSEWriter(c, ctrl.logger)
    defer writer.Close()

    // 5. 创建任务上下文
    ctx, cancel := context.WithTimeout(c.Request.Context(), ctrl.config.MaxStreamTime)
    defer cancel()

    // 注册任务
    ctrl.registerTask(taskID, cancel, userID, conversationID)
    defer ctrl.unregisterTask(taskID, conversationID)

    // 记录日志
    ctrl.logger.Info("用户重新生成消息",
        zap.String("task_id", taskID),
        zap.Uint("user_id", userID),
        zap.Uint("conversation_id", conversationID),
    )

    // 6. 获取会话的最后一条用户消息
    lastUserMsg, err := ctrl.messageSvc.GetLatestMessage(conversationID)
    if err != nil {
        ctrl.logger.Error("获取最后一条消息失败",
            zap.String("task_id", taskID),
            zap.Error(err),
        )
        writer.Send(EventMessageError, ErrorData{
            Code:    ErrCodeInternalError,
            Message: "获取消息失败",
        })
        writer.Send(EventDone, nil)
        return
    }

    if lastUserMsg.Role != "user" {
        writer.Send(EventMessageError, ErrorData{
            Code:    ErrCodeInvalidRequest,
            Message: "最后一条消息不是用户消息",
        })
        writer.Send(EventDone, nil)
        return
    }

    // 7. 删除该消息之后的所有 Assistant 消息
    // TODO: 实现删除逻辑

    // 8. 构建上下文并重新生成
    // 复用 Stream 方法的核心逻辑
    ctrl.processStreamWithContent(ctx, writer, conversationID, lastUserMsg.Content, "", taskID)
}

// Edit 编辑消息（SSE 接口）
// POST /api/v1/chat/edit
func (ctrl *Controller) Edit(c *gin.Context) {
    var req EditRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidRequest, "请求参数无效: "+err.Error()))
        return
    }

    userID := middleware.GetUserID(c)
    conversationID := req.ConversationID
    messageID := req.MessageID
    taskID := GenerateTaskID()

    // 1. 检查用户并发任务限制
    if err := ctrl.checkUserTaskLimit(userID); err != nil {
        c.JSON(http.StatusTooManyRequests, ErrorResponse(ErrCodeTooManyRequests, err.Error()))
        return
    }

    // 2. 取消会话的当前任务
    ctrl.cancelConversationTask(conversationID)

    // 3. 设置 SSE 响应头
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")

    // 4. 创建 SSE 写入器
    writer := NewSSEWriter(c, ctrl.logger)
    defer writer.Close()

    // 5. 创建任务上下文
    ctx, cancel := context.WithTimeout(c.Request.Context(), ctrl.config.MaxStreamTime)
    defer cancel()

    // 注册任务
    ctrl.registerTask(taskID, cancel, userID, conversationID)
    defer ctrl.unregisterTask(taskID, conversationID)

    // 记录日志
    ctrl.logger.Info("用户编辑消息",
        zap.String("task_id", taskID),
        zap.Uint("user_id", userID),
        zap.Uint("conversation_id", conversationID),
        zap.Uint("message_id", messageID),
        zap.String("new_content", req.NewContent),
    )

    // 6. 获取原始消息
    originalMsg, err := ctrl.messageSvc.GetMessage(messageID, conversationID)
    if err != nil {
        ctrl.logger.Error("获取原始消息失败",
            zap.String("task_id", taskID),
            zap.Error(err),
        )
        writer.Send(EventMessageError, ErrorData{
            Code:    ErrCodeInternalError,
            Message: "获取消息失败",
        })
        writer.Send(EventDone, nil)
        return
    }

    if originalMsg == nil {
        writer.Send(EventMessageError, ErrorData{
            Code:    ErrCodeMessageNotFound,
            Message: "消息不存在",
        })
        writer.Send(EventDone, nil)
        return
    }

    // 7. 更新消息内容
    // TODO: 实现更新逻辑

    // 8. 删除该消息之后的所有消息
    // TODO: 实现删除逻辑

    // 9. 构建上下文并重新生成
    ctrl.processStreamWithContent(ctx, writer, conversationID, req.NewContent, "", taskID)
}

// processStreamWithContent 处理流式对话（核心逻辑）
func (ctrl *Controller) processStreamWithContent(ctx context.Context, writer *SSEWriter, conversationID uint, content string, agentSlug string, taskID string) {
    // 1. 构建上下文
    query := &contextmgr.HistoryQuery{
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

    // 2. 创建用户消息并持久化
    userMessage := &schema.Message{
        Role:    schema.User,
        Content: content,
    }
    userMsgID, err := ctrl.contextMgr.PersistUserMessage(ctx, conversationID, userMessage)
    if err != nil {
        ctrl.logger.Error("保存用户消息失败",
            zap.String("task_id", taskID),
            zap.Error(err),
        )
    }
    _ = userMsgID

    // 3. 组装 Prompt
    messages := ctrl.contextMgr.BuildPrompt(ctx, window, userMessage)

    // 4. 获取 LLM 客户端
    llmClient, err := ctrl.getLLMClient(agentSlug, "")
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

    // 5. 生成消息ID
    messageID := fmt.Sprintf("msg_%s", uuid.New().String()[:8])

    // 6. 启动心跳
    stopHeartbeat := ctrl.heartbeat.Start(ctx, writer, messageID)
    defer ctrl.heartbeat.Stop(stopHeartbeat)

    // 7. 发送 message.start 事件
    writer.Send(EventMessageStart, MessageStartData{
        MessageID:      messageID,
        ConversationID: conversationID,
        Model:          "",
    })

    // 8. 调用 LLM 流式接口
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

    // 9. 读取流式输出并推送事件
    var fullContent string
    var index int
    startTime := time.Now()

    for {
        chunk, err := streamReader.Recv()
        if err != nil {
            if err.Error() == "EOF" {
                break
            }
            if ctx.Err() == context.Canceled {
                writer.Send(EventMessageCancelled, MessageCancelledData{
                    MessageID: messageID,
                    Reason:    "user_cancel",
                })
                writer.Send(EventDone, nil)
                return
            }
            if ctx.Err() == context.DeadlineExceeded {
                writer.Send(EventMessageError, ErrorData{
                    Code:    ErrCodeTimeout,
                    Message: "请求超时",
                })
                writer.Send(EventDone, nil)
                return
            }
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

        if chunk.Content != "" {
            fullContent += chunk.Content
            writer.Send(EventMessageDelta, MessageDeltaData{
                MessageID: messageID,
                Content:   chunk.Content,
                Index:     index,
            })
            index++
        }
    }

    // 10. 持久化 Assistant 消息
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

    // 11. 发送 message.complete 事件
    duration := time.Since(startTime)
    writer.Send(EventMessageComplete, MessageCompleteData{
        MessageID:    messageID,
        Content:      fullContent,
        TokenCount:   0,
        FinishReason: "stop",
    })

    ctrl.logger.Info("流式输出完成",
        zap.String("task_id", taskID),
        zap.String("message_id", messageID),
        zap.Duration("duration", duration),
    )

    // 12. 发送 done 事件
    writer.Send(EventDone, nil)
}

// UploadFile 上传文件
// POST /api/v1/chat/upload
func (ctrl *Controller) UploadFile(c *gin.Context) {
    var req FileUploadRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidRequest, "请求参数无效: "+err.Error()))
        return
    }

    userID := middleware.GetUserID(c)
    conversationID := req.ConversationID

    // 1. 校验会话权限
    // TODO: 调用 ConversationService 校验会话属于当前用户

    // 2. 校验文件大小
    maxSize := ctrl.getMaxFileSize(req.FileType)
    if req.FileSize > maxSize {
        c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse(ErrCodeFileTooLarge,
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

    c.JSON(http.StatusOK, SuccessResponse(FileUploadResponse{
        FileID:    fileID,
        FileName:  req.FileName,
        FileSize:  req.FileSize,
        FileType:  req.FileType,
        UploadURL: uploadURL,
    }))
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

// ProcessFile 处理已上传的文件
// POST /api/v1/chat/process-file
func (ctrl *Controller) ProcessFile(c *gin.Context) {
    var req struct {
        FileID uint `json:"file_id" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidRequest, "请求参数无效: "+err.Error()))
        return
    }

    userID := middleware.GetUserID(c)

    // 1. 获取文件信息
    // TODO: 从文件服务获取文件信息

    // 2. 发送 file.process_start 事件
    // 注意：这个API不是SSE，所以需要通过其他方式通知前端
    // 可以通过 Redis Pub/Sub 或 WebSocket 推送

    // 3. 处理文件
    // TODO: 根据文件类型进行处理
    // - 图片：OCR 或视觉模型
    // - 文档：解析提取文本
    // - 代码：语法分析

    // 4. 记录日志
    ctrl.logger.Info("文件处理请求",
        zap.Uint("user_id", userID),
        zap.Uint("file_id", req.FileID),
    )

    c.JSON(http.StatusOK, SuccessResponse(map[string]string{
        "message": "文件处理请求已接受",
    }))
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
    apiKey := "" // 从配置中获取
    baseURL := "" // 从配置中获取

    return ctrl.llmFactory(context.Background(), provider, modelName, apiKey, baseURL, 60)
}
```

---

# 6 SSE 配置 (config.go)

```go
package sse

import "time"

// DefaultConfig 返回默认 SSE 配置
func DefaultConfig() *SSEConfig {
    return &SSEConfig{
        MaxStreamTime:     5 * time.Minute, // 单次流式最大5分钟
        HeartbeatInterval: 15 * time.Second, // 心跳间隔：15秒
    }
}
```

---

# 7 心跳管理 (heartbeat.go)

## 7.1 为什么需要心跳

在流式输出过程中，如果 LLM 处理时间较长（如调用工具、知识库检索），可能会出现：

1. **代理超时**：Nginx、CDN 等反向代理默认有 60 秒超时
2. **浏览器超时**：某些浏览器会关闭长时间无数据的连接
3. **用户焦虑**：用户看不到任何反馈，以为连接断开

**心跳的作用**：定期发送空事件，保持连接活跃，给用户信心。

## 7.2 心跳机制设计

```
用户发送消息
    │
    ├── 启动心跳协程（每15秒发送一次）
    │
    ├── LLM 流式输出
    │   ├── 推送 message.delta
    │   └── 重置心跳计时器
    │
    ├── 心跳发送
    │   └── 推送 heartbeat 事件
    │
    └── 流式结束
        └── 停止心跳协程
```

## 7.3 实现（线程安全版本）

### ⚠️ 问题说明

**Gin 的 `c.Writer` 不是线程安全的！** 如果心跳协程和主协程并发写入，会导致 Data Race。

**解决方案**：使用 Channel 作为中转，所有写入操作由主协程统一处理。

### SSEWriter 实现

```go
package sse

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

// SSEEvent SSE 事件
type SSEEvent struct {
    Type EventType   `json:"type"`
    Data interface{} `json:"data"`
}

// SSEWriter 线程安全的 SSE 写入器
type SSEWriter struct {
    c       *gin.Context
    eventCh chan SSEEvent
    done    chan struct{}
    once    sync.Once
    logger  *zap.Logger
}

// NewSSEWriter 创建 SSE 写入器
func NewSSEWriter(c *gin.Context, logger *zap.Logger) *SSEWriter {
    w := &SSEWriter{
        c:       c,
        eventCh: make(chan SSEEvent, 100), // 缓冲 channel
        done:    make(chan struct{}),
        logger:  logger,
    }
    go w.writeLoop()
    return w
}

// writeLoop 写入循环（单线程处理所有写入）
func (w *SSEWriter) writeLoop() {
    for {
        select {
        case <-w.done:
            return
        case event, ok := <-w.eventCh:
            if !ok {
                return
            }
            w.writeEvent(event)
        }
    }
}

// writeEvent 写入单个事件
func (w *SSEWriter) writeEvent(event SSEEvent) {
    jsonData, err := json.Marshal(event)
    if err != nil {
        w.logger.Error("序列化事件失败", zap.Error(err))
        return
    }

    // 检查连接是否已关闭
    if w.c.Writer.Written() {
        return
    }

    _, err = fmt.Fprintf(w.c.Writer, "event: %s\ndata: %s\n\n", event.Type, string(jsonData))
    if err != nil {
        w.logger.Debug("写入事件失败（连接可能已关闭）", zap.Error(err))
        return
    }

    w.c.Writer.Flush()
}

// Send 发送事件（线程安全）
func (w *SSEWriter) Send(eventType EventType, data interface{}) {
    event := SSEEvent{
        Type: eventType,
        Data: data,
    }

    select {
    case w.eventCh <- event:
    case <-w.done:
        // 连接已关闭，丢弃事件
    default:
        // Channel 满了，丢弃事件（避免阻塞）
        w.logger.Warn("SSE 事件队列已满，丢弃事件",
            zap.String("event_type", string(eventType)),
        )
    }
}

// Close 关闭写入器
func (w *SSEWriter) Close() {
    w.once.Do(func() {
        close(w.done)
        close(w.eventCh)
    })
}

// HeartbeatManager 心跳管理器
type HeartbeatManager struct {
    interval time.Duration
    logger   *zap.Logger
}

// NewHeartbeatManager 创建心跳管理器
func NewHeartbeatManager(interval time.Duration, logger *zap.Logger) *HeartbeatManager {
    return &HeartbeatManager{
        interval: interval,
        logger:   logger,
    }
}

// Start 启动心跳
// 返回一个停止信号 channel
func (hm *HeartbeatManager) Start(ctx context.Context, writer *SSEWriter, messageID string) <-chan struct{} {
    stop := make(chan struct{})

    go func() {
        ticker := time.NewTicker(hm.interval)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                // 上下文取消，停止心跳
                return
            case <-stop:
                // 外部停止信号
                return
            case <-ticker.C:
                // 发送心跳事件（通过 SSEWriter，线程安全）
                writer.Send(EventHeartbeat, HeartbeatData{
                    MessageID: messageID,
                    Timestamp: time.Now().Unix(),
                })
            }
        }
    }()

    return stop
}

// Stop 停止心跳
func (hm *HeartbeatManager) Stop(stop <-chan struct{}) {
    select {
    case stop <- struct{}{}:
    default:
    }
}

// HeartbeatData 心跳数据
type HeartbeatData struct {
    MessageID string `json:"message_id"`
    Timestamp int64  `json:"timestamp"`
}
```

## 7.4 使用方式

在 Controller 的 Stream 方法中：

```go
func (ctrl *Controller) Stream(c *gin.Context) {
    // 1. 创建 SSE 写入器（线程安全）
    writer := NewSSEWriter(c, ctrl.logger)
    defer writer.Close()

    // 2. 设置 SSE 响应头（必须在第一次写入前）
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")

    // 3. 启动心跳
    stopHeartbeat := ctrl.heartbeat.Start(ctx, writer, messageID)
    defer ctrl.heartbeat.Stop(stopHeartbeat)

    // 4. 所有事件通过 writer.Send() 发送（线程安全）
    writer.Send(EventMessageStart, MessageStartData{...})
    writer.Send(EventMessageDelta, MessageDeltaData{...})
    writer.Send(EventMessageComplete, MessageCompleteData{...})
    writer.Send(EventDone, nil)
}
```

## 7.5 心跳事件格式

```
event: heartbeat
data: {"type":"heartbeat","data":{"message_id":"msg_456","timestamp":1690000000}}
```

前端收到心跳事件后，可以选择：
- 不显示给用户（静默心跳）
- 显示"AI正在思考..."提示
- 更新最后活跃时间

---

# 7 资源保护策略

| 场景 | 处理方式 | 配置 |
|------|----------|------|
| 同一会话新消息到达 | 自动取消旧消息处理 | - |
| 单次流式超时 | context.WithTimeout 自动取消 | 5 分钟 |
| LLM 调用失败 | 发送 error 事件并关闭连接 | - |
| 用户主动取消 | POST /cancel 取消 context | - |
| 上下文构建失败 | 发送 error 事件并关闭连接 | - |
| 代理超时 | 心跳机制保持连接活跃 | 15 秒 |
| 用户并发过多 | 限制单用户最大并发任务数 | 5 个 |
| 文件上传失败 | 发送 file.upload_error 事件 | - |
| 文件处理失败 | 发送 file.process_error 事件 | - |
| 文件大小超限 | 拒绝上传，返回错误 | 10MB |

---

## 4.9 文件处理流程

### 文件上传流程

```
用户选择文件
    │
    ├── 前端调用 POST /api/v1/chat/upload
    │   ├── 传入：conversation_id, file_type, file_name, file_size
    │   └── 返回：file_id, upload_url
    │
    ├── 前端上传文件到预签名URL
    │   ├── PUT upload_url
    │   └── 上传完成后，通知后端
    │
    ├── 后端处理文件
    │   ├── 根据文件类型选择处理方式
    │   │   ├── 图片：OCR 或视觉模型
    │   │   ├── 文档：解析提取文本
    │   │   ├── 代码：语法分析
    │   │   ├── 音频：语音识别
    │   │   └── 视频：关键帧提取
    │   │
    │   ├── 推送 file.process_start 事件
    │   │
    │   ├── 处理文件内容
    │   │
    │   └── 推送 file.process_done 事件
    │       └── 返回：content（提取的文本）
    │
    └── 用户发送消息时，附带 file_ids
        └── 上下文构建时，将文件内容嵌入到消息中
```

### 文件内容嵌入上下文

```go
// 构建包含文件内容的消息
func buildMessageWithFiles(content string, fileContents []string) string {
    if len(fileContents) == 0 {
        return content
    }

    // 将文件内容附加到消息中
    var parts []string
    parts = append(parts, content)

    for i, fc := range fileContents {
        parts = append(parts, fmt.Sprintf("\n\n--- 文件 %d 内容 ---\n%s", i+1, fc))
    }

    return strings.Join(parts, "")
}
```

### 文件处理错误处理

| 错误场景 | 错误码 | 处理方式 |
|----------|--------|----------|
| 文件大小超限 | `FILE_TOO_LARGE` | 拒绝上传，返回错误 |
| 文件类型不支持 | `UNSUPPORTED_FILE_TYPE` | 拒绝上传，返回错误 |
| 文件解析失败 | `FILE_PARSE_FAILED` | 发送 file.process_error 事件 |
| OCR 识别失败 | `OCR_FAILED` | 发送 file.process_error 事件 |
| 存储服务不可用 | `STORAGE_UNAVAILABLE` | 发送 file.upload_error 事件 |

---

# 8 API 路由定义

```go
// internal/api/v1/chat/routes.go

package chat

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册聊天相关路由
func RegisterRoutes(r *gin.RouterGroup, ctrl *Controller) {
    chat := r.Group("/chat")
    {
        // 流式对话（SSE）
        chat.POST("/stream", ctrl.Stream)

        // 取消生成
        chat.POST("/cancel", ctrl.Cancel)

        // 重新生成消息
        chat.POST("/regenerate", ctrl.Regenerate)

        // 编辑消息
        chat.POST("/edit", ctrl.Edit)

        // 文件上传
        chat.POST("/upload", ctrl.UploadFile)

        // 文件处理
        chat.POST("/process-file", ctrl.ProcessFile)
    }
}
```

**API 端点说明**：

| 端点 | 方法 | 说明 | 请求体 |
|------|------|------|--------|
| `/api/v1/chat/stream` | POST | 流式对话（SSE） | `StreamRequest` |
| `/api/v1/chat/cancel` | POST | 取消生成 | `CancelRequest` |
| `/api/v1/chat/regenerate` | POST | 重新生成消息 | `RegenerateRequest` |
| `/api/v1/chat/edit` | POST | 编辑消息 | `EditRequest` |
| `/api/v1/chat/upload` | POST | 上传文件 | `FileUploadRequest` |
| `/api/v1/chat/process-file` | POST | 处理文件 | `{ file_id: uint }` |

---

# 9 与其他模块的关系

| 模块 | 关系 | 说明 |
|------|------|------|
| **Task 4 上下文管理** | 调用 | FetchContext、BuildPrompt、PersistMessage |
| **LLM 抽象层** | 调用 | Stream() 获取流式输出 |
| **会话 CRUD** | 依赖 | 校验会话权限 |
| **消息 CRUD** | 调用 | 保存用户消息和 Assistant 消息 |
| **文件服务** | 调用 | 文件上传、处理、存储 |
| **Task 6 短期记忆** | 后续集成 | 流式结束后更新记忆 |
| **心跳管理器** | 内部调用 | HeartbeatManager 保持连接活跃 |
| **错误码定义** | 内部使用 | 统一错误码和HTTP状态码映射 |
| **前端聊天 UI** | 输出 | SSE 事件接收方 |

---

# 9 前端接入示例

```javascript
// 当前会话的SSE连接
let currentConnection = null;

// 发送消息并接收 SSE 事件
async function sendMessage(conversationId, content, agentSlug) {
  // 如果当前会话有正在进行的连接，先关闭它（同一会话串行处理）
  if (currentConnection && currentConnection.conversationId === conversationId) {
    currentConnection.controller.abort();
    currentConnection = null;
  }

  const controller = new AbortController();
  currentConnection = { conversationId, controller };

  try {
    const response = await fetch('/api/v1/chat/stream', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: JSON.stringify({
        conversation_id: conversationId,
        content: content,
        agent_slug: agentSlug,
      }),
      signal: controller.signal,
    });

    const reader = response.body.getReader();
    const decoder = new TextDecoder();

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      const text = decoder.decode(value);
      const lines = text.split('\n');

      for (const line of lines) {
        if (line.startsWith('event: ')) {
          const eventType = line.slice(7);
          // 处理事件类型
        } else if (line.startsWith('data: ')) {
          const data = JSON.parse(line.slice(6));
          // 处理事件数据
          handleSSEEvent(data);
        }
      }
    }
  } finally {
    if (currentConnection && currentConnection.conversationId === conversationId) {
      currentConnection = null;
    }
  }
}

// 处理 SSE 事件
function handleSSEEvent(event) {
  switch (event.type) {
    case 'message.start':
      // 消息开始
      break;
    case 'message.delta':
      // 增量内容，追加到 UI
      appendToMessage(event.data.content);
      break;
    case 'message.complete':
      // 消息完成
      finalizeMessage(event.data);
      break;
    case 'tool_call.start':
      // 工具调用开始，显示加载状态
      showToolLoading(event.data.tool_name);
      break;
    case 'tool_call.end':
      // 工具调用结束
      hideToolLoading(event.data.tool_name);
      break;
    case 'rag.start':
      // RAG 检索开始，显示检索状态
      showRAGLoading(event.data.query);
      break;
    case 'rag.done':
      // RAG 检索完成
      hideRAGLoading();
      break;
    case 'file.upload_start':
      // 文件上传开始
      showFileUpload(event.data.file_name);
      break;
    case 'file.upload_progress':
      // 文件上传进度
      updateFileProgress(event.data.file_id, event.data.progress);
      break;
    case 'file.upload_complete':
      // 文件上传完成
      completeFileUpload(event.data.file_id, event.data.file_url);
      break;
    case 'file.upload_error':
      // 文件上传失败
      showFileError(event.data.file_name, event.data.error);
      break;
    case 'file.process_start':
      // 文件处理开始
      showFileProcessing(event.data.file_name, event.data.process);
      break;
    case 'file.process_done':
      // 文件处理完成
      completeFileProcessing(event.data.file_id, event.data.content);
      break;
    case 'file.process_error':
      // 文件处理失败
      showFileError(event.data.file_name, event.data.error);
      break;
    case 'heartbeat':
      // 心跳事件，可以更新最后活跃时间或显示思考状态
      updateLastActiveTime(event.data.timestamp);
      break;
    case 'message.cancelled':
      // 消息已取消（用户中断）
      handleCancelled(event.data.message_id, event.data.reason);
      break;
    case 'message.regenerate':
      // 消息重新生成
      handleRegenerate(event.data.original_message_id);
      break;
    case 'message.edited':
      // 消息已编辑
      handleEdited(event.data.original_message_id, event.data.deleted_messages);
      break;
    case 'message.error':
      // 错误处理
      showError(event.data.code, event.data.message);
      break;
    case 'done':
      // 流结束
      break;
  }
}

// 取消生成
async function cancelGeneration(taskId) {
  await fetch('/api/v1/chat/cancel', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({ task_id: taskId }),
  });
}

// 重新生成消息
async function regenerateMessage(conversationId) {
  const response = await fetch('/api/v1/chat/regenerate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({ conversation_id: conversationId }),
  });

  const result = await response.json();
  if (result.success) {
    // 发起新的流式请求
    await sendMessage(conversationId, lastUserMessage, agentSlug);
  }
}

// 编辑消息
async function editMessage(conversationId, messageId, newContent) {
  const response = await fetch('/api/v1/chat/edit', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({
      conversation_id: conversationId,
      message_id: messageId,
      new_content: newContent,
    }),
  });

  const result = await response.json();
  if (result.success) {
    // 发起新的流式请求
    await sendMessage(conversationId, newContent, agentSlug);
  }
}

// 处理取消事件
function handleCancelled(messageId, reason) {
  console.log(`消息 ${messageId} 已取消，原因: ${reason}`);
  // 显示取消提示
  showCancelledMessage(reason);
}

// 处理重新生成事件
function handleRegenerate(originalMessageId) {
  console.log(`消息 ${originalMessageId} 正在重新生成`);
  // 清除旧的 Assistant 消息
  clearAssistantMessage(originalMessageId);
  // 显示重新生成提示
  showRegeneratingMessage();
}

// 处理编辑事件
function handleEdited(originalMessageId, deletedMessages) {
  console.log(`消息 ${originalMessageId} 已编辑，删除了 ${deletedMessages} 条消息`);
  // 更新消息显示
  updateMessageDisplay(originalMessageId);
  // 清除后续消息
  clearMessagesAfter(originalMessageId);
}

// 文件上传
async function uploadFile(conversationId, file, fileType) {
  // 1. 请求上传URL
  const response = await fetch('/api/v1/chat/upload', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({
      conversation_id: conversationId,
      file_type: fileType,
    }),
  });

  const result = await response.json();
  if (!result.success) {
    showError('文件上传失败', result.error);
    return null;
  }

  const uploadData = result.data;

  // 2. 上传文件到预签名URL
  const uploadResponse = await fetch(uploadData.upload_url, {
    method: 'PUT',
    body: file,
    headers: {
      'Content-Type': file.type,
    },
  });

  if (!uploadResponse.ok) {
    showError('文件上传失败', '上传到存储服务失败');
    return null;
  }

  return uploadData.file_id;
}

// 处理文件上传事件
function showFileUpload(fileName) {
  console.log(`开始上传文件: ${fileName}`);
  // 显示上传进度条
}

function updateFileProgress(fileId, progress) {
  console.log(`文件 ${fileId} 上传进度: ${progress * 100}%`);
  // 更新进度条
}

function completeFileUpload(fileId, fileUrl) {
  console.log(`文件 ${fileId} 上传完成: ${fileUrl}`);
  // 隐藏进度条，显示文件预览
}

function showFileProcessing(fileName, processType) {
  console.log(`开始处理文件: ${fileName}, 类型: ${processType}`);
  // 显示处理中状态
}

function completeFileProcessing(fileId, content) {
  console.log(`文件 ${fileId} 处理完成, 内容长度: ${content.length}`);
  // 显示提取的文本内容
}

function showFileError(fileName, error) {
  console.error(`文件 ${fileName} 处理失败: ${error}`);
  // 显示错误提示
}

// 发送带文件的消息
async function sendMessageWithFiles(conversationId, content, agentSlug, fileIds) {
  // 如果当前会话有正在进行的连接，先关闭它
  if (currentConnection && currentConnection.conversationId === conversationId) {
    currentConnection.controller.abort();
    currentConnection = null;
  }

  const controller = new AbortController();
  currentConnection = { conversationId, controller };

  try {
    const response = await fetch('/api/v1/chat/stream', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: JSON.stringify({
        conversation_id: conversationId,
        content: content,
        agent_slug: agentSlug,
        file_ids: fileIds, // 附带文件ID列表
      }),
      signal: controller.signal,
    });

    // ... 后续处理与 sendMessage 相同
  } finally {
    if (currentConnection && currentConnection.conversationId === conversationId) {
      currentConnection = null;
    }
  }
}
```

---

# 10 后续扩展

1. **✅ 心跳机制**：已实现，每15秒发送心跳，防止代理超时
2. **✅ 同一会话串行处理**：已实现，新消息到达时自动取消旧消息
3. **✅ 用户操作事件**：已实现中断、重新生成、编辑消息功能
4. **✅ 文件上传支持**：已实现文件上传、处理、状态推送
5. **断线重连**：前端断线后恢复消息流（需要事件ID支持）
6. **消息队列**：通过 Redis Pub/Sub 解耦（多实例部署时）
7. **多标签页支持**：任务ID隔离，避免冲突
8. **Token 统计**：从 LLM 响应中获取 Token 消耗
9. **流式中断恢复**：支持从指定 index 继续推送
10. **消息版本控制**：支持查看消息的历史版本
11. **批量操作**：支持批量删除、批量重新生成
12. **文件预览**：支持图片、PDF、代码等文件的在线预览
13. **多模态支持**：支持图片、音频、视频等多模态输入
