# Qavor API

基于 Go 语言和 Gin 框架构建的标准三层 MVC 架构后端项目基础框架。

## 项目简介

是一个轻量级、可扩展的后端项目基础架构，采用标准的 MVC 分层设计，提供了完整的用户认证、日志管理、数据库连接等功能，适合作为其他项目的起点。

### 架构特点

- **应用启动器模式**：通过 `internal/app/app.go` 统一管理应用初始化流程，main.go 仅需 20 行代码
- **清晰的分层架构**：Controller → Service → Repository，职责明确
- **依赖注入**：所有层次通过构造函数注入，便于测试和扩展
- **接口隔离**：Service 和 Repository 层使用接口定义，支持多种实现

## 技术栈

- **Web 框架**: Gin v1.9.1
- **ORM**: GORM v1.25.5
- **数据库**: MySQL 8.0+
- **缓存**: Redis
- **配置管理**: Viper
- **日志**: Zap + Lumberjack
- **认证**: JWT
- **密码加密**: bcrypt

## 项目结构

```
Qavor/
├── cmd/
│   └── server/
│       └── main.go                    # 主程序入口
├── internal/                           # 私有应用代码
│   ├── app/                           # 应用启动器
│   │   └── app.go                     # 应用初始化和启动
│   ├── api/                           # API 层
│   │   ├── v1/                        # API v1 版本
│   │   │   ├── user/                  # 用户模块
│   │   │   └── auth/                  # 认证模块
│   │   └── router.go                  # 路由注册
│   ├── service/                       # 业务逻辑层
│   ├── repository/                    # 数据访问层
│   ├── model/                         # 数据模型
│   │   ├── entity/                    # 数据库实体
│   │   └── dto/                       # 数据传输对象
│   └── middleware/                    # 中间件
├── pkg/                               # 公共工具包
│   ├── config/                        # 配置管理
│   ├── logger/                        # 日志系统
│   ├── database/                      # 数据库管理
│   ├── jwt/                           # JWT 工具
│   ├── response/                      # 统一响应
│   ├── errors/                        # 错误处理
│   └── utils/                         # 工具函数
├── configs/                           # 配置文件
│   ├── config.yaml                    # 主配置
│   └── config.yaml.example            # 配置示例
├── scripts/                           # 脚本
│   └── migrate.sql                    # 数据库迁移
├── docs/                              # 项目文档
│   ├── ARCHITECTURE.md                # 架构设计
│   ├── DEVELOPMENT.md                 # 开发指南
│   └── API.md                         # API 文档
├── .env.example                       # 环境变量示例
├── .gitignore
├── Makefile                           # 构建命令
├── go.mod                             # Go 模块
└── README.md                          # 项目说明
```

## 快速开始

### 前置要求

- Go 1.21+
- MySQL 8.0+
- Redis 6.0+

### 安装步骤

1. 克隆项目
```bash
git clone https://github.com/your-username/Qavor.git
cd Qavor
```

2. 安装依赖
```bash
go mod tidy
```

3. 配置数据库
```bash
# config.yaml.example重命名 config.yaml
# 编辑 configs/config.yaml，修改数据库连接信息
# 或使用环境变量覆盖
export MYSQL_PASSWORD=your_password
```

4. 创建数据库
```bash
mysql -u root -p < scripts/migrate.sql
```

5. 运行项目
```bash
go run cmd/server/main.go
```

或使用 Makefile：
```bash
make run
```

### 配置说明

配置文件位于 `configs/config.yaml`，主要配置项：

- `app`: 应用配置（名称、版本、端口）
- `database`: 数据库配置（MySQL、Redis）
- `jwt`: JWT 认证配置
- `log`: 日志配置
- `cors`: 跨域配置

支持通过环境变量覆盖敏感配置：
- `MYSQL_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`

### API 端点

#### 健康检查
```
GET /api/v1/health
```

#### 用户认证
```
POST /api/v1/auth/register  # 用户注册
POST /api/v1/auth/login     # 用户登录
POST /api/v1/auth/refresh   # 刷新 Token
```

#### 用户管理（需认证）
```
GET /api/v1/user/profile    # 获取用户信息
PUT /api/v1/user/profile    # 更新用户信息
POST /api/v1/user/password  # 修改密码
GET /api/v1/user/list       # 获取用户列表（分页）
```

### API 测试

#### 用户注册
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "123456",
    "email": "test@example.com"
  }'
```

#### 用户登录
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "123456"
  }'
```

#### 获取用户信息（需要 Token）
```bash
curl http://localhost:8080/api/v1/user/profile \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

#### 获取用户列表（需要 Token，分页）
```bash
curl "http://localhost:8080/api/v1/user/list?page=1&size=10" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

## Makefile 命令

```bash
make build        # 编译项目
make run          # 运行项目
make test         # 运行测试
make clean        # 清理构建文件
make deps         # 下载依赖
make fmt          # 格式化代码
make vet          # 代码静态检查
make help         # 显示帮助
```

## 开发指南

详细的开发指南请查看：
- [架构设计](docs/ARCHITECTURE.md)
- [开发指南](docs/DEVELOPMENT.md)
- [API 文档](docs/API.md)

## 核心特性

- ✅ 标准 MVC 三层架构
- ✅ 应用启动器（App Launcher）- 统一管理初始化流程
- ✅ JWT 认证
- ✅ 统一响应格式
- ✅ 统一错误处理
- ✅ 分页查询支持
- ✅ 结构化日志（Zap）
- ✅ 日志轮转（Lumberjack）
- ✅ 配置管理（Viper）
- ✅ 数据库迁移（GORM AutoMigrate）
- ✅ 优雅关闭（Graceful Shutdown）
- ✅ CORS 支持
- ✅ 中间件系统（Logger/Recovery/CORS/Auth）