# 会话与消息 CRUD 设计文档

## 概述

设计并实现对话会话（Conversation）和消息（Message）的 CRUD 功能，包括数据表设计、Repository 层、Service 层、Controller/API 层，并与现有系统集成。

## 需求总结

### 功能需求

1. **会话管理**
   - 创建会话
   - 获取会话详情
   - 更新会话（标题）
   - 删除会话（软删除）
   - 获取会话列表（分页）
   - 关闭会话
   - 归档会话

2. **消息管理**
   - 创建消息
   - 获取消息详情
   - 更新消息
   - 删除消息（软删除）
   - 获取消息列表（分页）
   - 获取最新消息

### 非功能需求

1. **用户级隔离**：会话绑定到用户，用户只能访问自己的会话
2. **软删除**：删除操作使用软删除，保留数据便于恢复和审计
3. **会话状态**：支持 active/closed/archived 三种状态
4. **消息排序**：默认按创建时间倒序
5. **分页支持**：列表查询支持分页

## 实体层设计

### 现有实体（已定义）

#### Conversation 实体

```go
type Conversation struct {
    BaseEntity
    Title      string         `gorm:"size:255;not null" json:"title"`
    Status     string         `gorm:"size:20;default:active" json:"status"` // active/closed/archived
    UserID     uint           `gorm:"index" json:"user_id"`
    ModelID    *uint          `json:"model_id"`
    ExtraData  JSON           `json:"extra_data"`
    DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"` // 软删除
}
```

#### Message 实体

```go
type Message struct {
    BaseEntity
    ConversationID uint           `gorm:"index" json:"conversation_id"`
    Role           string         `gorm:"size:20;not null" json:"role"` // user/assistant/system/tool
    Content        string         `gorm:"type:text" json:"content"`
    MessageType    string         `gorm:"size:20;default:text" json:"message_type"` // text/tool_call/tool_result
    Usage          JSON           `json:"usage"` // token 使用统计
    ModelName      string         `gorm:"size:100" json:"model_name"`
    ToolCalls      JSON           `json:"tool_calls"`
    ImageContent   string         `gorm:"type:text" json:"image_content"`
    DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at"` // 软删除
}
```

### 设计决策

1. **软删除**：通过 `gorm.DeletedAt` 字段实现
2. **用户绑定**：通过 `UserID` 字段关联用户
3. **会话状态**：支持 active/closed/archived 三种状态
4. **消息排序**：默认按创建时间倒序

## Repository 层设计

### ConversationRepository

```go
type ConversationRepository interface {
    Create(conversation *entity.Conversation) error
    FindByID(id uint) (*entity.Conversation, error)
    FindByIDAndUserID(id, userID uint) (*entity.Conversation, error)
    Update(conversation *entity.Conversation) error
    Delete(id uint) error
    ListByUserID(userID uint, offset, limit int) ([]entity.Conversation, int64, error)
    ListByUserIDWithStatus(userID uint, status string, offset, limit int) ([]entity.Conversation, int64, error)
}
```

### MessageRepository

```go
type MessageRepository interface {
    Create(message *entity.Message) error
    FindByID(id uint) (*entity.Message, error)
    FindByIDAndConversationID(id, conversationID uint) (*entity.Message, error)
    Update(message *entity.Message) error
    Delete(id uint) error
    ListByConversationID(conversationID uint, offset, limit int) ([]entity.Message, int64, error)
    ListByConversationIDWithRole(conversationID uint, role string, offset, limit int) ([]entity.Message, int64, error)
    CountByConversationID(conversationID uint) (int64, error)
    GetLatestByConversationID(conversationID uint) (*entity.Message, error)
}
```

### 设计决策

1. **用户级查询**：`FindByIDAndUserID` 确保用户只能访问自己的会话
2. **软删除支持**：GORM 自动处理 `DeletedAt` 字段
3. **分页支持**：通过 `offset` 和 `limit` 参数
4. **消息排序**：默认按创建时间倒序（在 SQL 中实现）

## Service 层设计

### ConversationService

```go
type ConversationService interface {
    CreateConversation(userID uint, req *request.CreateConversationRequest) (*dto.ConversationResponse, error)
    GetConversation(id, userID uint) (*dto.ConversationResponse, error)
    UpdateConversation(id, userID uint, req *request.UpdateConversationRequest) (*dto.ConversationResponse, error)
    DeleteConversation(id, userID uint) error
    ListConversations(userID uint, req *request.ConversationListRequest) (*dto.ConversationListResponse, error)
    CloseConversation(id, userID uint) (*dto.ConversationResponse, error)
    ArchiveConversation(id, userID uint) (*dto.ConversationResponse, error)
}
```

### MessageService

```go
type MessageService interface {
    CreateMessage(userID uint, req *request.CreateMessageRequest) (*dto.MessageResponse, error)
    GetMessage(id, conversationID uint) (*dto.MessageResponse, error)
    UpdateMessage(id, conversationID uint, req *request.UpdateMessageRequest) (*dto.MessageResponse, error)
    DeleteMessage(id, conversationID uint) error
    ListMessages(conversationID uint, req *request.MessageListRequest) (*dto.MessageListResponse, error)
    GetLatestMessage(conversationID uint) (*dto.MessageResponse, error)
}
```

### 设计决策

1. **用户级权限**：所有方法都接收 `userID` 参数，确保用户只能访问自己的数据
2. **会话状态管理**：提供 `CloseConversation` 和 `ArchiveConversation` 方法
3. **DTO 转换**：Service 层负责将实体转换为响应 DTO
4. **业务逻辑**：Service 层处理业务规则（如创建消息时更新会话统计）

## Controller/API 层设计

### Conversation Controller

```go
// 路由设计
POST   /api/v1/conversations          // 创建会话
GET    /api/v1/conversations          // 获取会话列表
GET    /api/v1/conversations/:id      // 获取会话详情
PUT    /api/v1/conversations/:id      // 更新会话
DELETE /api/v1/conversations/:id      // 删除会话
PUT    /api/v1/conversations/:id/close    // 关闭会话
PUT    /api/v1/conversations/:id/archive  // 归档会话
```

### Message Controller

```go
// 路由设计
POST   /api/v1/conversations/:conversation_id/messages          // 创建消息
GET    /api/v1/conversations/:conversation_id/messages          // 获取消息列表
GET    /api/v1/conversations/:conversation_id/messages/:id      // 获取消息详情
PUT    /api/v1/conversations/:conversation_id/messages/:id      // 更新消息
DELETE /api/v1/conversations/:conversation_id/messages/:id      // 删除消息
GET    /api/v1/conversations/:conversation_id/messages/latest   // 获取最新消息
```

### 设计决策

1. **RESTful 设计**：遵循 RESTful API 设计规范
2. **嵌套路由**：消息路由嵌套在会话路由下，体现资源层级关系
3. **用户认证**：通过中间件获取当前用户 ID
4. **响应格式**：统一使用 `pkg/response` 包的响应格式

## 错误处理与验证

### 错误码设计

```go
// 会话相关错误码
CodeConversationNotFound     = 40001  // 会话不存在
CodeConversationAccessDenied = 40002  // 无权访问会话
CodeConversationStatusInvalid = 40003 // 会话状态无效

// 消息相关错误码
CodeMessageNotFound         = 40011  // 消息不存在
CodeMessageAccessDenied     = 40012  // 无权访问消息
CodeMessageRoleInvalid      = 40013  // 消息角色无效
```

### 验证规则

#### CreateConversationRequest

```go
type CreateConversationRequest struct {
    Title   string `json:"title" binding:"required,min=1,max=255"`
    ModelID *uint  `json:"model_id" binding:"omitempty"`
}
```

#### UpdateConversationRequest

```go
type UpdateConversationRequest struct {
    Title string `json:"title" binding:"omitempty,min=1,max=255"`
}
```

#### CreateMessageRequest

```go
type CreateMessageRequest struct {
    Role        string      `json:"role" binding:"required,oneof=user assistant system tool"`
    Content     string      `json:"content" binding:"required"`
    MessageType string      `json:"message_type" binding:"omitempty,oneof=text tool_call tool_result"`
    ImageContent string     `json:"image_content" binding:"omitempty"`
    ExtraMetadata entity.JSON `json:"extra_metadata" binding:"omitempty"`
}
```

### 设计决策

1. **错误码规范**：使用5位数字错误码，按模块分段
2. **输入验证**：使用 Gin 的 binding 标签进行参数验证
3. **权限验证**：在 Service 层验证用户权限
4. **状态验证**：在 Service 层验证会话状态转换

## 与现有系统集成

### 依赖注入

在 `app.go` 中添加依赖注入：

```go
// initDependencies 初始化依赖注入
func (a *App) initDependencies() {
    // 创建 Repository
    conversationRepo := repository.NewConversationRepository(a.postgresDB)
    messageRepo := repository.NewMessageRepository(a.postgresDB)
    // ... 其他 Repository

    // 创建 Service
    conversationSvc := service.NewConversationService(conversationRepo)
    messageSvc := service.NewMessageService(messageRepo, conversationRepo)
    // ... 其他 Service

    // 创建 Router
    a.router = api.NewRouter(
        // ... 其他参数
        conversationSvc,
        messageSvc,
    )
}
```

### 路由注册

在 `router.go` 中添加路由：

```go
// Router 路由
type Router struct {
    // ... 其他控制器
    conversationCtrl *conversation.Controller
    messageCtrl      *message.Controller
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
    // ... 其他路由

    // 会话和消息路由
    r.conversationCtrl.RegisterRoutes(v1)
    r.messageCtrl.RegisterRoutes(v1)
}
```

### 数据库迁移

在 `app.go` 的 `initDatabase` 中，`Conversation` 和 `Message` 实体已包含在迁移列表中，无需额外配置。

### 设计决策

1. **依赖注入**：遵循项目现有的依赖注入模式
2. **路由注册**：使用 `RegisterRoutes` 方法统一管理路由
3. **数据库迁移**：利用 GORM 的 AutoMigrate 功能
4. **中间件**：复用现有的认证、日志、CORS 中间件

## 文件结构

```
internal/
├── repository/
│   ├── conversation_repository.go
│   └── message_repository.go
├── service/
│   ├── conversation_service.go
│   └── message_service.go
├── api/v1/
│   ├── conversation/
│   │   ├── controller.go
│   │   └── router.go
│   └── message/
│       ├── controller.go
│       └── router.go
├── model/
│   ├── entity/
│   │   ├── conversation.go      (已存在)
│   │   └── message.go           (已存在)
│   └── dto/
│       ├── request/
│       │   ├── conversation.go  (已存在)
│       │   └── message.go       (已存在)
│       └── response/
│           ├── conversation.go  (已存在)
│           └── message.go       (已存在)
```

## 实现顺序

1. **Repository 层**：实现 `conversation_repository.go` 和 `message_repository.go`
2. **Service 层**：实现 `conversation_service.go` 和 `message_service.go`
3. **Controller 层**：实现 `conversation/controller.go` 和 `message/controller.go`
4. **Router 层**：实现 `conversation/router.go` 和 `message/router.go`
5. **集成**：更新 `app.go` 和 `router.go` 添加依赖注入和路由注册
6. **测试**：编写单元测试和集成测试

