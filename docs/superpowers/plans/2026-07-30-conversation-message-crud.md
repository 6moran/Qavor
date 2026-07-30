# 会话与消息 CRUD 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现对话会话和消息的完整 CRUD，包括 Repository、Service、Controller 三层，并与现有系统集成。

**Architecture:** 标准分层架构（Repository → Service → Controller），遵循项目现有模式。会话通过 AgentID 关联 Agent，消息嵌套在会话下，支持分页和状态管理（active/archived/deleted）。

**Tech Stack:** Go, GORM, Gin, PostgreSQL, `pkg/errors`, `pkg/response`, `pkg/validator`

## Global Constraints

- 遵循项目现有代码风格和命名规范
- 使用 `pkg/errors` 包定义错误码（40001-40013 范围）
- 使用 `pkg/response` 包返回统一响应格式
- 使用 `pkg/validator` 包处理参数验证错误
- 会话状态使用 "active/archived/deleted" 三态管理
- 所有列表接口默认按 `created_at DESC` 排序
- 消息列表默认倒序（最新消息在前）
- 会话通过 AgentID 关联 Agent，通过 ThreadID (UUID) 唯一标识
- 消息通过 ConversationID 关联会话
- **Redis Stream 缓存**：消息创建后发布到 Redis Stream，支持实时推送和 SSE 流式响应

---

## 文件结构

### 新建文件

| 文件路径 | 职责 |
|---------|------|
| `internal/repository/conversation_repository.go` | 会话数据访问层 |
| `internal/repository/message_repository.go` | 消息数据访问层 |
| `internal/service/conversation_service.go` | 会话业务逻辑层 |
| `internal/service/message_service.go` | 消息业务逻辑层 |
| `internal/api/v1/conversation/controller.go` | 会话 HTTP 控制器 |
| `internal/api/v1/conversation/router.go` | 会话路由注册 |
| `internal/api/v1/message/controller.go` | 消息 HTTP 控制器 |
| `internal/api/v1/message/router.go` | 消息路由注册 |

### 修改文件

| 文件路径 | 修改内容 |
|---------|---------|
| `pkg/errors/code.go` | 添加会话/消息错误码 (40001-40013) |
| `internal/api/router.go` | 添加 conversation/message 控制器和路由注册 |
| `internal/app/app.go` | 添加 conversation/message 依赖注入 |

### 已存在文件（只读参考）

| 文件路径 | 用途 |
|---------|------|
| `internal/model/entity/conversation.go` | 会话实体定义 |
| `internal/model/entity/message.go` | 消息实体定义 |
| `internal/model/dto/request/conversation.go` | 会话请求 DTO |
| `internal/model/dto/request/message.go` | 消息请求 DTO |
| `internal/model/dto/response/conversation.go` | 会话响应 DTO |
| `internal/model/dto/response/message.go` | 消息响应 DTO |

---

### Task 1: 添加会话与消息错误码

**Files:**
- Modify: `pkg/errors/code.go:56-61`

**Interfaces:**
- Consumes: 无
- Produces: 6 个错误码常量 + 对应错误消息，后续所有 Task 中使用

- [ ] **Step 1: 在 `pkg/errors/code.go` 末尾添加错误码常量**

在 `// 模型提供商错误 5xxx` 区块之后追加：

```go
	// 会话错误 40xxx
	CodeConversationNotFound      = 40001
	CodeConversationAccessDenied  = 40002
	CodeConversationStatusInvalid = 40003

	// 消息错误 400xx
	CodeMessageNotFound     = 40011
	CodeMessageAccessDenied = 40012
	CodeMessageRoleInvalid  = 40013
```

- [ ] **Step 2: 在 `codeMessages` map 中添加对应消息**

在 `code.go` 的 `codeMessages` map 末尾追加：

```go
		CodeConversationNotFound:      "会话不存在",
		CodeConversationAccessDenied:  "无权访问会话",
		CodeConversationStatusInvalid: "会话状态无效",
		CodeMessageNotFound:           "消息不存在",
		CodeMessageAccessDenied:       "无权访问消息",
		CodeMessageRoleInvalid:        "消息角色无效",
```

- [ ] **Step 3: 编译验证**

Run: `go build ./pkg/errors/...`
Expected: 编译通过，无错误

- [ ] **Step 4: Commit**

```bash
git add pkg/errors/code.go
git commit -m "feat(errors): 添加会话与消息错误码"
```

---

### Task 2: 实现 ConversationRepository

**Files:**
- Create: `internal/repository/conversation_repository.go`

**Interfaces:**
- Consumes: `internal/model/entity/conversation.go` (已存在)
- Produces: `ConversationRepository` 接口，被 `ConversationService` 消费

- [ ] **Step 1: 创建 `internal/repository/conversation_repository.go`**

```go
package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// ConversationRepository 会话仓储接口
type ConversationRepository interface {
	Create(conversation *entity.Conversation) error
	FindByID(id uint) (*entity.Conversation, error)
	FindByIDAndUserID(id, userID uint) (*entity.Conversation, error)
	Update(conversation *entity.Conversation) error
	Delete(id uint) error
	ListByUserID(userID uint, offset, limit int) ([]entity.Conversation, int64, error)
	ListByUserIDWithStatus(userID uint, status string, offset, limit int) ([]entity.Conversation, int64, error)
}

// conversationRepository 会话仓储实现
type conversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository 创建会话仓储
func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepository{db: db}
}

// Create 创建会话
func (r *conversationRepository) Create(conversation *entity.Conversation) error {
	return r.db.Create(conversation).Error
}

// FindByID 根据 ID 查找会话
func (r *conversationRepository) FindByID(id uint) (*entity.Conversation, error) {
	var conversation entity.Conversation
	err := r.db.First(&conversation, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

// FindByIDAndUserID 根据 ID 和用户 ID 查找会话（用户级权限校验）
func (r *conversationRepository) FindByIDAndUserID(id, userID uint) (*entity.Conversation, error) {
	var conversation entity.Conversation
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&conversation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

// Update 更新会话
func (r *conversationRepository) Update(conversation *entity.Conversation) error {
	return r.db.Save(conversation).Error
}

// Delete 删除会话（软删除）
func (r *conversationRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Conversation{}, id).Error
}

// ListByUserID 根据用户 ID 分页获取会话列表
func (r *conversationRepository) ListByUserID(userID uint, offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.Model(&entity.Conversation{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}

// ListByUserIDWithStatus 根据用户 ID 和状态分页获取会话列表
func (r *conversationRepository) ListByUserIDWithStatus(userID uint, status string, offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.Model(&entity.Conversation{}).Where("user_id = ? AND status = ?", userID, status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/repository/...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add internal/repository/conversation_repository.go
git commit -m "feat(repo): 实现 ConversationRepository"
```

---

### Task 3: 实现 MessageRepository

**Files:**
- Create: `internal/repository/message_repository.go`

**Interfaces:**
- Consumes: `internal/model/entity/message.go` (已存在)
- Produces: `MessageRepository` 接口，被 `MessageService` 消费

- [ ] **Step 1: 创建 `internal/repository/message_repository.go`**

```go
package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// MessageRepository 消息仓储接口
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

// messageRepository 消息仓储实现
type messageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建消息仓储
func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}

// Create 创建消息
func (r *messageRepository) Create(message *entity.Message) error {
	return r.db.Create(message).Error
}

// FindByID 根据 ID 查找消息
func (r *messageRepository) FindByID(id uint) (*entity.Message, error) {
	var message entity.Message
	err := r.db.First(&message, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &message, nil
}

// FindByIDAndConversationID 根据 ID 和会话 ID 查找消息
func (r *messageRepository) FindByIDAndConversationID(id, conversationID uint) (*entity.Message, error) {
	var message entity.Message
	err := r.db.Where("id = ? AND conversation_id = ?", id, conversationID).First(&message).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &message, nil
}

// Update 更新消息
func (r *messageRepository) Update(message *entity.Message) error {
	return r.db.Save(message).Error
}

// Delete 删除消息（软删除）
func (r *messageRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Message{}, id).Error
}

// ListByConversationID 根据会话 ID 分页获取消息列表（倒序）
func (r *messageRepository) ListByConversationID(conversationID uint, offset, limit int) ([]entity.Message, int64, error) {
	var messages []entity.Message
	var total int64

	query := r.db.Model(&entity.Message{}).Where("conversation_id = ?", conversationID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

// ListByConversationIDWithRole 根据会话 ID 和角色分页获取消息列表（倒序）
func (r *messageRepository) ListByConversationIDWithRole(conversationID uint, role string, offset, limit int) ([]entity.Message, int64, error) {
	var messages []entity.Message
	var total int64

	query := r.db.Model(&entity.Message{}).Where("conversation_id = ? AND role = ?", conversationID, role)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

// CountByConversationID 统计会话下的消息数量
func (r *messageRepository) CountByConversationID(conversationID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Message{}).Where("conversation_id = ?", conversationID).Count(&count).Error
	return count, err
}

// GetLatestByConversationID 获取会话下最新的一条消息
func (r *messageRepository) GetLatestByConversationID(conversationID uint) (*entity.Message, error) {
	var message entity.Message
	err := r.db.Where("conversation_id = ?", conversationID).Order("created_at DESC").First(&message).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &message, nil
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/repository/...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add internal/repository/message_repository.go
git commit -m "feat(repo): 实现 MessageRepository"
```

---

### Task 4: 实现 ConversationService

**Files:**
- Create: `internal/service/conversation_service.go`

**Interfaces:**
- Consumes: `repository.ConversationRepository` (Task 2), `model/dto/request/conversation.go` (已存在), `model/dto/response/conversation.go` (已存在)
- Produces: `ConversationService` 接口，被 `conversation.Controller` 消费

- [ ] **Step 1: 创建 `internal/service/conversation_service.go`**

```go
package service

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/errors"
)

// ConversationService 会话服务接口
type ConversationService interface {
	CreateConversation(userID uint, req *request.CreateConversationRequest) (*dto.ConversationResponse, error)
	GetConversation(id, userID uint) (*dto.ConversationResponse, error)
	UpdateConversation(id, userID uint, req *request.UpdateConversationRequest) (*dto.ConversationResponse, error)
	DeleteConversation(id, userID uint) error
	ListConversations(userID uint, req *request.ConversationListRequest) (*dto.ConversationListResponse, error)
	CloseConversation(id, userID uint) (*dto.ConversationResponse, error)
	ArchiveConversation(id, userID uint) (*dto.ConversationResponse, error)
}

// conversationService 会话服务实现
type conversationService struct {
	conversationRepo repository.ConversationRepository
}

// NewConversationService 创建会话服务
func NewConversationService(conversationRepo repository.ConversationRepository) ConversationService {
	return &conversationService{
		conversationRepo: conversationRepo,
	}
}

// CreateConversation 创建会话
func (s *conversationService) CreateConversation(userID uint, req *request.CreateConversationRequest) (*dto.ConversationResponse, error) {
	conversation := &entity.Conversation{
		Title:  req.Title,
		Status: "active",
		UserID: userID,
	}

	if req.ModelID != nil {
		conversation.ModelID = req.ModelID
	}

	if err := s.conversationRepo.Create(conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "创建会话失败")
	}

	return s.toResponse(conversation), nil
}

// GetConversation 获取会话详情
func (s *conversationService) GetConversation(id, userID uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByIDAndUserID(id, userID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	return s.toResponse(conversation), nil
}

// UpdateConversation 更新会话
func (s *conversationService) UpdateConversation(id, userID uint, req *request.UpdateConversationRequest) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByIDAndUserID(id, userID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	if req.Title != "" {
		conversation.Title = req.Title
	}

	if err := s.conversationRepo.Update(conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "更新会话失败")
	}

	return s.toResponse(conversation), nil
}

// DeleteConversation 删除会话
func (s *conversationService) DeleteConversation(id, userID uint) error {
	conversation, err := s.conversationRepo.FindByIDAndUserID(id, userID)
	if err != nil {
		return err
	}
	if conversation == nil {
		return errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	return s.conversationRepo.Delete(id)
}

// ListConversations 获取会话列表
func (s *conversationService) ListConversations(userID uint, req *request.ConversationListRequest) (*dto.ConversationListResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	var conversations []entity.Conversation
	var total int64
	var err error

	if req.Status != "" {
		conversations, total, err = s.conversationRepo.ListByUserIDWithStatus(userID, req.Status, offset, pageSize)
	} else {
		conversations, total, err = s.conversationRepo.ListByUserID(userID, offset, pageSize)
	}

	if err != nil {
		return nil, err
	}

	items := make([]dto.ConversationResponse, 0, len(conversations))
	for _, conv := range conversations {
		items = append(items, *s.toResponse(&conv))
	}

	return &dto.ConversationListResponse{
		Total: total,
		Items: items,
	}, nil
}

// CloseConversation 关闭会话
func (s *conversationService) CloseConversation(id, userID uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByIDAndUserID(id, userID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	if conversation.Status == "closed" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已关闭")
	}
	if conversation.Status == "archived" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已归档，无法关闭")
	}

	conversation.Status = "closed"
	if err := s.conversationRepo.Update(conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "关闭会话失败")
	}

	return s.toResponse(conversation), nil
}

// ArchiveConversation 归档会话
func (s *conversationService) ArchiveConversation(id, userID uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByIDAndUserID(id, userID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	if conversation.Status == "archived" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已归档")
	}

	conversation.Status = "archived"
	if err := s.conversationRepo.Update(conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "归档会话失败")
	}

	return s.toResponse(conversation), nil
}

// toResponse 将实体转换为响应 DTO
func (s *conversationService) toResponse(conv *entity.Conversation) *dto.ConversationResponse {
	resp := &dto.ConversationResponse{
		ID:        conv.ID,
		Title:     conv.Title,
		Status:    conv.Status,
		UserID:    conv.UserID,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}
	if conv.ModelID != nil {
		resp.ModelID = conv.ModelID
	}
	if conv.ExtraData != nil {
		resp.ExtraData = conv.ExtraData
	}
	return resp
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/service/...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add internal/service/conversation_service.go
git commit -m "feat(service): 实现 ConversationService"
```

---

### Task 5: 实现 MessageService（含 Redis Stream 发布）

**Files:**
- Create: `internal/service/message_service.go`

**Interfaces:**
- Consumes: `repository.MessageRepository` (Task 3), `repository.ConversationRepository` (Task 2), `model/dto/request/message.go` (已存在), `model/dto/response/message.go` (已存在), `redis.Client` (已初始化)
- Produces: `MessageService` 接口，被 `message.Controller` 消费；消息创建后发布到 Redis Stream `stream:conversation:{conversation_id}:messages`

- [ ] **Step 1: 创建 `internal/service/message_service.go`**

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/errors"

	"github.com/redis/go-redis/v9"
)

// MessageService 消息服务接口
type MessageService interface {
	CreateMessage(userID uint, req *request.CreateMessageRequest) (*dto.MessageResponse, error)
	GetMessage(id, conversationID uint) (*dto.MessageResponse, error)
	UpdateMessage(id, conversationID uint, req *request.UpdateMessageRequest) (*dto.MessageResponse, error)
	DeleteMessage(id, conversationID uint) error
	ListMessages(conversationID uint, req *request.MessageListRequest) (*dto.MessageListResponse, error)
	GetLatestMessage(conversationID uint) (*dto.MessageResponse, error)
}

// messageService 消息服务实现
type messageService struct {
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	redis            *redis.Client
}

// NewMessageService 创建消息服务
func NewMessageService(messageRepo repository.MessageRepository, conversationRepo repository.ConversationRepository, redis *redis.Client) MessageService {
	return &messageService{
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		redis:            redis,
	}
}

// CreateMessage 创建消息
func (s *messageService) CreateMessage(userID uint, req *request.CreateMessageRequest) (*dto.MessageResponse, error) {
	// 校验会话存在且属于当前用户
	conv, err := s.conversationRepo.FindByIDAndUserID(req.ConversationID, userID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	// 校验会话状态
	if conv.Status != "active" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已关闭或归档，无法发送消息")
	}

	// 设置默认消息类型
	messageType := req.MessageType
	if messageType == "" {
		messageType = "text"
	}

	message := &entity.Message{
		ConversationID: req.ConversationID,
		Role:           req.Role,
		Content:        req.Content,
		MessageType:    messageType,
		ImageContent:   req.ImageContent,
		ExtraMetadata:  req.ExtraMetadata,
	}

	if err := s.messageRepo.Create(message); err != nil {
		return nil, errors.New(errors.CodeInternalError, "创建消息失败")
	}

	// 发布到 Redis Stream，支持实时推送
	if err := s.publishToRedisStream(message); err != nil {
		// Redis 发布失败不影响主流程，仅记录日志
		fmt.Printf("Redis Stream 发布失败: %v\n", err)
	}

	return s.toResponse(message), nil
}

// publishToRedisStream 发布消息到 Redis Stream
func (s *messageService) publishToRedisStream(message *entity.Message) error {
	if s.redis == nil {
		return nil
	}

	streamKey := fmt.Sprintf("stream:conversation:%d:messages", message.ConversationID)

	// 序列化消息为 JSON
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// 发布到 Redis Stream
	ctx := context.Background()
	err = s.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"message_id": message.ID,
			"role":       message.Role,
			"content":    message.Content,
			"data":       string(data),
		},
	}).Err()

	return err
}

// GetMessage 获取消息详情
func (s *messageService) GetMessage(id, conversationID uint) (*dto.MessageResponse, error) {
	message, err := s.messageRepo.FindByIDAndConversationID(id, conversationID)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, errors.New(errors.CodeMessageNotFound, "消息不存在")
	}

	return s.toResponse(message), nil
}

// UpdateMessage 更新消息
func (s *messageService) UpdateMessage(id, conversationID uint, req *request.UpdateMessageRequest) (*dto.MessageResponse, error) {
	message, err := s.messageRepo.FindByIDAndConversationID(id, conversationID)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, errors.New(errors.CodeMessageNotFound, "消息不存在")
	}

	if req.Content != "" {
		message.Content = req.Content
	}
	if req.ExtraMetadata != nil {
		message.ExtraMetadata = req.ExtraMetadata
	}

	if err := s.messageRepo.Update(message); err != nil {
		return nil, errors.New(errors.CodeInternalError, "更新消息失败")
	}

	return s.toResponse(message), nil
}

// DeleteMessage 删除消息
func (s *messageService) DeleteMessage(id, conversationID uint) error {
	message, err := s.messageRepo.FindByIDAndConversationID(id, conversationID)
	if err != nil {
		return err
	}
	if message == nil {
		return errors.New(errors.CodeMessageNotFound, "消息不存在")
	}

	return s.messageRepo.Delete(id)
}

// ListMessages 获取消息列表
func (s *messageService) ListMessages(conversationID uint, req *request.MessageListRequest) (*dto.MessageListResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	var messages []entity.Message
	var total int64
	var err error

	if req.Role != "" {
		messages, total, err = s.messageRepo.ListByConversationIDWithRole(conversationID, req.Role, offset, pageSize)
	} else {
		messages, total, err = s.messageRepo.ListByConversationID(conversationID, offset, pageSize)
	}

	if err != nil {
		return nil, err
	}

	items := make([]dto.MessageResponse, 0, len(messages))
	for _, msg := range messages {
		items = append(items, *s.toResponse(&msg))
	}

	return &dto.MessageListResponse{
		Total: total,
		Items: items,
	}, nil
}

// GetLatestMessage 获取最新消息
func (s *messageService) GetLatestMessage(conversationID uint) (*dto.MessageResponse, error) {
	message, err := s.messageRepo.GetLatestByConversationID(conversationID)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, errors.New(errors.CodeMessageNotFound, "会话暂无消息")
	}

	return s.toResponse(message), nil
}

// toResponse 将实体转换为响应 DTO
func (s *messageService) toResponse(msg *entity.Message) *dto.MessageResponse {
	return &dto.MessageResponse{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		Role:           msg.Role,
		Content:        msg.Content,
		MessageType:    msg.MessageType,
		ModelName:      msg.ModelName,
		ImageContent:   msg.ImageContent,
		CreatedAt:      msg.CreatedAt,
		UpdatedAt:      msg.UpdatedAt,
	}
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/service/...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add internal/service/message_service.go
git commit -m "feat(service): 实现 MessageService"
```

---

### Task 6: 实现 Conversation Controller 和 Router

**Files:**
- Create: `internal/api/v1/conversation/controller.go`
- Create: `internal/api/v1/conversation/router.go`

**Interfaces:**
- Consumes: `service.ConversationService` (Task 4)
- Produces: `conversation.Controller`，被 `api/router.go` 消费

- [ ] **Step 1: 创建 `internal/api/v1/conversation/controller.go`**

```go
package conversation

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Controller 会话控制器
type Controller struct {
	conversationService service.ConversationService
}

// NewController 创建会话控制器
func NewController(conversationService service.ConversationService) *Controller {
	return &Controller{conversationService: conversationService}
}

// CreateConversation 创建会话
func (ctrl *Controller) CreateConversation(c *gin.Context) {
	var req request.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	userID, _ := c.Get("user_id")
	resp, err := ctrl.conversationService.CreateConversation(userID.(uint), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetConversation 获取会话详情
func (ctrl *Controller) GetConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	userID, _ := c.Get("user_id")
	resp, err := ctrl.conversationService.GetConversation(uint(id), userID.(uint))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// UpdateConversation 更新会话
func (ctrl *Controller) UpdateConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	var req request.UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	userID, _ := c.Get("user_id")
	resp, err := ctrl.conversationService.UpdateConversation(uint(id), userID.(uint), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// DeleteConversation 删除会话
func (ctrl *Controller) DeleteConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	userID, _ := c.Get("user_id")
	if err := ctrl.conversationService.DeleteConversation(uint(id), userID.(uint)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ListConversations 获取会话列表
func (ctrl *Controller) ListConversations(c *gin.Context) {
	var req request.ConversationListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	userID, _ := c.Get("user_id")
	resp, err := ctrl.conversationService.ListConversations(userID.(uint), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// CloseConversation 关闭会话
func (ctrl *Controller) CloseConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	userID, _ := c.Get("user_id")
	resp, err := ctrl.conversationService.CloseConversation(uint(id), userID.(uint))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// ArchiveConversation 归档会话
func (ctrl *Controller) ArchiveConversation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的ID")
		return
	}

	userID, _ := c.Get("user_id")
	resp, err := ctrl.conversationService.ArchiveConversation(uint(id), userID.(uint))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}
```

- [ ] **Step 2: 创建 `internal/api/v1/conversation/router.go`**

```go
package conversation

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册会话路由
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	conversations := router.Group("/conversations")
	{
		conversations.POST("", ctrl.CreateConversation)
		conversations.GET("", ctrl.ListConversations)
		conversations.GET("/:id", ctrl.GetConversation)
		conversations.PUT("/:id", ctrl.UpdateConversation)
		conversations.DELETE("/:id", ctrl.DeleteConversation)
		conversations.PUT("/:id/close", ctrl.CloseConversation)
		conversations.PUT("/:id/archive", ctrl.ArchiveConversation)
	}
}
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/api/v1/conversation/...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add internal/api/v1/conversation/
git commit -m "feat(api): 实现 Conversation Controller 和 Router"
```

---

### Task 7: 实现 Message Controller 和 Router

**Files:**
- Create: `internal/api/v1/message/controller.go`
- Create: `internal/api/v1/message/router.go`

**Interfaces:**
- Consumes: `service.MessageService` (Task 5)
- Produces: `message.Controller`，被 `api/router.go` 消费

- [ ] **Step 1: 创建 `internal/api/v1/message/controller.go`**

```go
package message

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Controller 消息控制器
type Controller struct {
	messageService service.MessageService
}

// NewController 创建消息控制器
func NewController(messageService service.MessageService) *Controller {
	return &Controller{messageService: messageService}
}

// CreateMessage 创建消息
func (ctrl *Controller) CreateMessage(c *gin.Context) {
	conversationIDStr := c.Param("conversation_id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	var req request.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}
	req.ConversationID = uint(conversationID)

	userID, _ := c.Get("user_id")
	resp, err := ctrl.messageService.CreateMessage(userID.(uint), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetMessage 获取消息详情
func (ctrl *Controller) GetMessage(c *gin.Context) {
	conversationIDStr := c.Param("conversation_id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的消息ID")
		return
	}

	resp, err := ctrl.messageService.GetMessage(uint(id), uint(conversationID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// UpdateMessage 更新消息
func (ctrl *Controller) UpdateMessage(c *gin.Context) {
	conversationIDStr := c.Param("conversation_id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的消息ID")
		return
	}

	var req request.UpdateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.messageService.UpdateMessage(uint(id), uint(conversationID), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// DeleteMessage 删除消息
func (ctrl *Controller) DeleteMessage(c *gin.Context) {
	conversationIDStr := c.Param("conversation_id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的消息ID")
		return
	}

	if err := ctrl.messageService.DeleteMessage(uint(id), uint(conversationID)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ListMessages 获取消息列表
func (ctrl *Controller) ListMessages(c *gin.Context) {
	conversationIDStr := c.Param("conversation_id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	var req request.MessageListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.messageService.ListMessages(uint(conversationID), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetLatestMessage 获取最新消息
func (ctrl *Controller) GetLatestMessage(c *gin.Context) {
	conversationIDStr := c.Param("conversation_id")
	conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的会话ID")
		return
	}

	resp, err := ctrl.messageService.GetLatestMessage(uint(conversationID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}
```

- [ ] **Step 2: 创建 `internal/api/v1/message/router.go`**

```go
package message

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册消息路由
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	messages := router.Group("/conversations/:conversation_id/messages")
	{
		messages.POST("", ctrl.CreateMessage)
		messages.GET("", ctrl.ListMessages)
		messages.GET("/latest", ctrl.GetLatestMessage)
		messages.GET("/:id", ctrl.GetMessage)
		messages.PUT("/:id", ctrl.UpdateMessage)
		messages.DELETE("/:id", ctrl.DeleteMessage)
	}
}
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/api/v1/message/...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add internal/api/v1/message/
git commit -m "feat(api): 实现 Message Controller 和 Router"
```

---

### Task 8: 集成到 Router 和 App

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/app/app.go`

**Interfaces:**
- Consumes: `conversation.Controller` (Task 6), `message.Controller` (Task 7)
- Produces: 完整的路由注册和依赖注入

- [ ] **Step 1: 修改 `internal/api/router.go`**

```go
package api

import (
	"Qavor/internal/api/v1/auth"
	"Qavor/internal/api/v1/conversation"
	knowledgebase "Qavor/internal/api/v1/knowledge_base"
	knowledgefile "Qavor/internal/api/v1/knowledge_file"
	"Qavor/internal/api/v1/message"
	"Qavor/internal/api/v1/model"
	"Qavor/internal/middleware"
	"Qavor/internal/service"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	authCtrl          *auth.Controller
	knowledgeBaseCtrl *knowledgebase.Controller
	knowledgeFileCtrl *knowledgefile.Controller
	modelCtrl         *model.Controller
	conversationCtrl  *conversation.Controller
	messageCtrl       *message.Controller
}

// NewRouter 创建路由
func NewRouter(
	authService service.AuthService,
	knowledgeBaseService service.KnowledgeBaseService,
	knowledgeFileService service.KnowledgeFileService,
	modelService service.ModelService,
	conversationService service.ConversationService,
	messageService service.MessageService,
) *Router {
	return &Router{
		authCtrl:          auth.NewController(authService),
		knowledgeBaseCtrl: knowledgebase.NewController(knowledgeBaseService),
		knowledgeFileCtrl: knowledgefile.NewController(knowledgeFileService),
		modelCtrl:         model.NewController(modelService),
		conversationCtrl:  conversation.NewController(conversationService),
		messageCtrl:       message.NewController(messageService),
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// 健康检查
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Qavor API is running",
		})
	})

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	{
		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 知识库路由
		r.knowledgeBaseCtrl.RegisterRoutes(v1)
		r.knowledgeFileCtrl.RegisterRoutes(v1)

		// 模型路由
		r.modelCtrl.RegisterRoutes(v1)

		// 会话路由
		r.conversationCtrl.RegisterRoutes(v1)

		// 消息路由
		r.messageCtrl.RegisterRoutes(v1)
	}
}
```

- [ ] **Step 2: 修改 `internal/app/app.go` 的 `initDependencies` 方法**

```go
// initDependencies 初始化依赖注入
func (a *App) initDependencies() {
	// 创建 Repository
	knowledgeBaseRepo := repository.NewKnowledgeBaseRepository(a.postgresDB)
	knowledgeFileRepo := repository.NewKnowledgeFileRepository(a.postgresDB)
	modelRepo := repository.NewModelRepository(a.postgresDB)
	conversationRepo := repository.NewConversationRepository(a.postgresDB)
	messageRepo := repository.NewMessageRepository(a.postgresDB)

	// 创建 Service
	authSvc := service.NewAuthService(a.cfg.Auth)
	modelSvc := service.NewModelService(modelRepo)
	knowledgeBaseSvc := service.NewKnowledgeBaseService(knowledgeBaseRepo)
	knowledgeFileSvc := service.NewKnowledgeFileService(knowledgeBaseRepo, knowledgeFileRepo, service.NewMinIOObjectStorage())
	conversationSvc := service.NewConversationService(conversationRepo)
	messageSvc := service.NewMessageService(messageRepo, conversationRepo, a.redis)

	// 创建 Router
	a.router = api.NewRouter(authSvc, knowledgeBaseSvc, knowledgeFileSvc, modelSvc, conversationSvc, messageSvc)
}
```

- [ ] **Step 3: 编译验证**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add internal/api/router.go internal/app/app.go
git commit -m "feat: 集成会话与消息模块到路由和依赖注入"
```

---

### Task 9: 端到端冒烟验证

**Files:**
- 无新建/修改文件

**Interfaces:**
- Consumes: 全部前序 Task
- Produces: 验证所有 API 端点可访问

- [ ] **Step 1: 启动应用**

Run: `go run main.go`
Expected: 应用启动成功，日志输出 "HTTP 服务器启动"

- [ ] **Step 2: 验证健康检查**

Run: `curl http://localhost:8080/api/v1/health`
Expected: `{"status":"ok","message":"Qavor API is running"}`

- [ ] **Step 3: 验证会话列表 API 可访问**

Run: `curl http://localhost:8080/api/v1/conversations`
Expected: 返回 JSON 响应（可能提示未授权，但路由可达）

- [ ] **Step 4: 验证消息列表 API 可访问**

Run: `curl http://localhost:8080/api/v1/conversations/1/messages`
Expected: 返回 JSON 响应（可能提示未授权，但路由可达）

- [ ] **Step 5: 停止应用并 Commit**

```bash
git add -A
git commit -m "chore: 完成会话与消息 CRUD 模块集成验证"
```
