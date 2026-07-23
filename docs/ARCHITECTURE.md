# 架构设计文档

## 概述

Qavor 采用标准的 MVC 三层架构设计，实现了清晰的分层结构和依赖注入。

## 架构分层

```
┌─────────────────────────────────────────┐
│          API Layer (Controller)          │  HTTP 请求处理
├─────────────────────────────────────────┤
│         Service Layer (Business)         │  业务逻辑处理
├─────────────────────────────────────────┤
│      Repository Layer (Data Access)      │  数据访问
├─────────────────────────────────────────┤
│         Database (MySQL + Redis)         │  数据存储
└─────────────────────────────────────────┘
```

## 层次说明

### 1. API 层 (Controller)

**位置**: `internal/api/`

**职责**:
- 处理 HTTP 请求和响应
- 参数绑定和验证
- 调用 Service 层处理业务
- 返回统一格式的响应

**示例**:
```go
func (ctrl *Controller) GetProfile(c *gin.Context) {
    userID := middleware.GetUserID(c)
    user, err := ctrl.userService.GetUserByID(userID)
    // ...
}
```

### 2. Service 层

**位置**: `internal/service/`

**职责**:
- 实现核心业务逻辑
- 事务管理
- 调用 Repository 层进行数据操作
- 业务规则验证

**示例**:
```go
func (s *userService) Register(req *request.RegisterRequest) error {
    // 业务验证
    exists, _ := s.userRepo.ExistsByUsername(req.Username)
    if exists {
        return errors.ErrUserAlreadyExists
    }
    // 业务处理
    // ...
}
```

### 3. Repository 层

**位置**: `internal/repository/`

**职责**:
- 封装数据访问逻辑
- CRUD 操作
- 数据查询和转换

**示例**:
```go
func (r *userRepository) FindByID(id uint) (*entity.User, error) {
    var user entity.User
    err := r.db.First(&user, id).Error
    // ...
}
```

## 设计模式

### 依赖注入

所有层之间通过构造函数注入依赖，便于测试和扩展。

```go
// Controller 注入 Service
func NewController(userService service.UserService) *Controller {
    return &Controller{userService: userService}
}

// Service 注入 Repository
func NewUserService(userRepo repository.UserRepository) UserService {
    return &userService{userRepo: userRepo}
}
```

### 接口隔离

Service 和 Repository 层使用接口定义，支持 Mock 测试和多种实现。

```go
// 定义接口
type UserRepository interface {
    FindByID(id uint) (*entity.User, error)
    Create(user *entity.User) error
}

// 实现接口
type userRepository struct {
    db *gorm.DB
}
```

## 核心组件

### 1. 配置管理

**位置**: `pkg/config/`

- 使用 Viper 加载 YAML 配置
- 支持环境变量覆盖
- 支持多环境配置（dev/prod）

### 2. 日志系统

**位置**: `pkg/logger/`

- 使用 Zap 结构化日志
- 集成 Lumberjack 实现日志轮转
- 支持控制台和文件输出

### 3. 数据库管理

**位置**: `pkg/database/`

- MySQL 连接池配置
- GORM 配置和初始化
- Redis 连接管理

### 4. 认证系统

**位置**: `pkg/jwt/`, `internal/middleware/auth.go`

- JWT Token 生成和验证
- 认证中间件
- 从 Token 提取用户信息

### 5. 错误处理

**位置**: `pkg/errors/`

- 自定义错误类型
- 统一错误码管理
- 业务错误和系统错误分离

### 6. 中间件系统

**位置**: `internal/middleware/`

- **Logger**: 记录请求日志
- **Recovery**: Panic 恢复
- **CORS**: 跨域处理
- **Auth**: JWT 认证

## 数据模型

### Entity (实体)

**位置**: `internal/model/entity/`

对应数据库表结构，使用 GORM 标签配置：

```go
type User struct {
    BaseEntity
    Username string `gorm:"type:varchar(50);uniqueIndex"`
    Password string `gorm:"type:varchar(255)"`
    // ...
}
```

### DTO (数据传输对象)

**位置**: `internal/model/dto/`

- **Request**: API 请求数据结构
- **Response**: API 响应数据结构

## 请求流程

```
HTTP Request
    ↓
[Middleware] → Recovery, Logger, CORS, Auth
    ↓
[Controller] → 参数绑定和验证
    ↓
[Service] → 业务逻辑处理
    ↓
[Repository] → 数据库操作
    ↓
Database
    ↓
[Response] → 统一响应格式
    ↓
HTTP Response
```

## 数据库设计

### 表结构

- **users**: 用户表
  - 基础字段：id, created_at, updated_at, deleted_at
  - 业务字段：username, password, email, nickname, avatar, status

### 索引设计

- 唯一索引：username, email
- 普通索引：deleted_at

## 安全设计

1. **密码加密**: 使用 bcrypt 加密存储
2. **JWT 认证**: Token 有效期控制
3. **CORS 配置**: 限制跨域访问
4. **参数验证**: 使用 validator 标签
5. **SQL 注入防护**: 使用 GORM 参数化查询

## 性能优化

1. **连接池**: MySQL 和 Redis 连接池配置
2. **日志轮转**: 避免日志文件过大
3. **优雅关闭**: 等待现有请求完成
4. **跳过默认事务**: GORM 配置提升性能
