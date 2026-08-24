# 开发指南

> 面向 Qavor 后端（Go）与前端（Vue 3）开发。本文档基于当前代码库实际约定编写。
> 最后核对日期：2026-08-20

## 1. 环境准备

| 依赖 | 版本/说明 |
| --- | --- |
| Go | 1.25+（`go.mod` 声明） |
| Node.js + pnpm | 前端构建（`frontend/`） |
| PostgreSQL | 需启用 pgvector、pg_trgm 扩展 |
| Redis | 队列（Streams）、Token 黑名单、短期记忆、SSE 心跳 |
| MinIO | 对象存储（文件/图片上传） |
| Python 3 | 文档解析子进程（`pkg/documentparser/python/`，依赖见其 requirements.txt） |

初始化数据库（首次）：

```bash
# 建库并启用扩展（以 Linux/macOS 为例）
psql -U postgres -c "CREATE DATABASE qavor;"
psql -U postgres -d qavor -c "CREATE EXTENSION IF NOT EXISTS vector;"
psql -U postgres -d qavor -c "CREATE EXTENSION IF NOT EXISTS pg_trgm;"

# 或执行迁移脚本（Makefile）
make migrate PGPASSWORD=xxx
```

## 2. 快速启动

```bash
# 安装依赖
go mod tidy

# 复制配置并填写数据库/Redis/MinIO/模型密钥
cp configs/config.yaml.example configs/config.yaml

# 启动后端（默认端口 8080）
make run

# 前端（另开终端）
cd frontend && pnpm install && pnpm dev
```

常用 Makefile 目标：

```bash
make build             # 编译到 bin/（注入版本信息）
make run               # 运行后端
make test              # 运行全部 Go 测试
make test-coverage     # 覆盖率 HTML 报告
make migrate           # 执行 scripts/migrate.sql
make fmt / make vet    # 格式化 / 静态检查
make frontend-build / frontend-dev / frontend-lint
```

## 3. 代码结构与分层约定

```
internal/
├── app/                  # 装配中心：所有依赖在此创建并注入（启动器模式）
├── api/
│   ├── router.go         # 全局中间件 + 各模块 RegisterRoutes 调用
│   └── v1/<module>/      # Controller + routes.go（HTTP 层）
├── service/              # 业务逻辑：<module>_interface.go + <module>_service.go
├── repository/           # 数据访问：<module>_interface.go + <module>_repository.go
├── model/
│   ├── entity/           # GORM 实体（建表结构）
│   └── dto/request|response/
├── middleware/           # Recovery / Logger / CORS / Auth
└── <domain>/             # 领域能力：rag / agent / memory / run / trace / sse / skill / mcp ...
pkg/                      # 通用基础设施：config / logger / database / minio / jwt / errors / response ...
```

**硬性约定**：

1. **依赖方向单向**：`api → service → repository/domain → entity/dto`，禁止反向依赖；`llm`、`embedding` 等底层包禁止导入上层（entity/service/database）。
2. **接口 + 实现分离**：Service 与 Repository 均定义接口（`*_interface.go`）供 Controller 依赖与测试 Mock，实现放同包独立文件。
3. **Controller 不写业务逻辑**：只做参数绑定、校验、调用 Service、返回统一响应（`pkg/response`）。
4. **新增依赖在 `internal/app/app.go` 的 `initDependencies()` 装配**，`main.go` 不动。
5. **领域模块可被多个 Service 复用**（如 `rag` 同时服务 knowledge、chat、agent 的 query_kb）。

## 4. 新增一个 API 模块（完整流程）

以新增 `knowledge` 模块为真实参照（`internal/api/v1/knowledge_base/`、`internal/service/knowledge_base_*`、`internal/repository/knowledge_base_*`）。

### 4.1 定义实体与 DTO

实体放 `internal/model/entity/`：

```go
// internal/model/entity/product.go
package entity

type Product struct {
	BaseEntity
	Name  string  `gorm:"type:varchar(100);not null" json:"name"`
	Price float64 `gorm:"type:decimal(10,2)" json:"price"`
}
```

请求/响应放 `internal/model/dto/request/`、`internal/model/dto/response/`：

```go
// internal/model/dto/request/product.go
package request

type CreateProductRequest struct {
	Name  string  `json:"name" binding:"required"`
	Price float64 `json:"price" binding:"required,gt=0"`
}
```

### 4.2 Repository：接口 + 实现

```go
// internal/repository/product_interface.go
package repository

type ProductRepository interface {
	Create(p *entity.Product) error
	FindByID(id uint) (*entity.Product, error)
	List(offset, limit int, keyword string) ([]*entity.Product, int64, error)
}
```

```go
// internal/repository/product_repository.go
package repository

type productRepository struct{ db *gorm.DB }

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(p *entity.Product) error {
	return r.db.Create(p).Error
}
```

> 数据库为 PostgreSQL：涉及 jsonb 用 `attributes->>'xxx'`，模糊检索用 `ILIKE`，向量检索见 `internal/repository/knowledge_chunk_repository.go`。

### 4.3 Service：接口 + 实现

```go
// internal/service/product_interface.go
package service

type ProductService interface {
	Create(req *request.CreateProductRequest) error
	GetByID(id uint) (*entity.Product, error)
	List(page, pageSize int) ([]*entity.Product, int64, error)
}
```

```go
// internal/service/product_service.go
package service

type productService struct {
	productRepo repository.ProductRepository
}

func NewProductService(productRepo repository.ProductRepository) ProductService {
	return &productService{productRepo: productRepo}
}

func (s *productService) Create(req *request.CreateProductRequest) error {
	return s.productRepo.Create(&entity.Product{Name: req.Name, Price: req.Price})
}
```

### 4.4 Controller + 路由

```go
// internal/api/v1/product/controller.go
package product

type Controller struct {
	productService service.ProductService
}

func NewController(productService service.ProductService) *Controller {
	return &Controller{productService: productService}
}

func (c *Controller) Create(ctx *gin.Context) {
	var req request.CreateProductRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(ctx, code, message)
		return
	}
	if err := c.productService.Create(&req); err != nil {
		response.BizError(ctx, err)
		return
	}
	response.Success(ctx, nil)
}
```

```go
// internal/api/v1/product/routes.go
package product

// 需认证的模块在 group 上挂 middleware.Auth()
func (c *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/products")
	group.Use(middleware.Auth())
	{
		group.POST("", c.Create)
		group.GET("", c.List)
	}
}
```

### 4.5 注册路由 + 装配

`internal/api/router.go`：Router 结构体加字段 → `NewRouter` 参数与赋值 → `Setup` 中调用 `RegisterRoutes`。

`internal/app/app.go`：`initDependencies()` 中创建 Repository → Service → Controller，传入 `api.NewRouter(...)`。若新增数据库表，加入 `initDatabase()` 的 `AutoMigrate` 列表（`config.yaml` 的 `database.auto_migrate: true` 时生效）。

### 4.6 前端 API 封装

```js
// frontend/src/apis/product_api.js
import { apiRequest } from './base';

export function listProducts(params) {
  const query = new URLSearchParams(params).toString();
  return apiRequest(`/api/products?${query}`);  // base.js 会自动归一化为 /api/v1/products
}
```

> **注意**：`base.js` 的 `apiRequest` 不处理 `params` 选项，GET 带参必须手动拼 `URLSearchParams`（参考 `workspace_api.js` 的 buildQuery 模式）。

## 5. 测试规范

- **Go 单测**：`go test ./...`（或 `make test`）。Service 层用 Mock Repository（接口化带来的收益），如 `knowledge_base_service_test.go`。
- **前端测试**：`frontend/test/unit/*.test.js`，用 `node --test` 运行。
- **提交前必须全绿**：先改代码 → 写/跑测试 → **测试通过才可提交**；测试失败必须先修复，禁止带红提交。
- **提交范围约定**：只提交代码文件；`docs/` 文件夹、测试文件（`frontend/test/`、`*_test.go`、测试数据）一律不提交。
- 提交信息：中文 Conventional Commits，如 `feat(rag): 增加混合检索 RRF 融合`。

## 6. 代码规范

### 命名

- 文件名：小写 + 下划线（`user_service.go`）；包名小写单词
- 接口：能力命名（`ProductRepository`）；实现：小写结构体（`productRepository`）
- 常量：`MAX_RETRY_COUNT` 风格

### 错误处理

统一使用 `pkg/errors` 错误码与 `pkg/response` 响应：

```go
// 业务错误（返回对应 code）
return errors.NewDefault(errors.CodeResourceNotFound)

// Controller 层
if err != nil {
	response.BizError(ctx, err)   // 业务错误
	response.InternalServerError(ctx)  // 系统错误
}
```

### 日志

统一 Zap（`pkg/logger`），关键操作与错误必须记录：

```go
logger.Info("知识库创建成功", zap.String("kb_id", kbID))
logger.Error("解析失败", zap.Error(err))
```

### 模型接入约定（重要）

- **Base URL 填写 API 根地址**（如 `https://api.siliconflow.cn/v1`），不要填完整 endpoint（如 `.../chat/completions`），否则内部拼路径会 404。
- 连接测试走 eino 链路；若响应非 JSON，错误特征为 `invalid character 'X' looking for beginning of value`（X 是错误页首字母）。

## 7. 调试技巧

1. **看日志**：`logs/app.log`（`config.yaml` 的 `log.level` 可调 debug）。
2. **链路追踪**：开启 `trace.enabled: true`（默认开），对话/检索/Agent 执行会记录 Trace，前端「追踪」页面按 `parent_span_id` 组装 Span 树排查。
3. **数据库**：开发时直接查 PostgreSQL；`auto_migrate: false` 时手工改表需谨慎。
4. **队列**：Redis Stream 消费状态可通过 `XRANGE` / `XINFO` 查看（`parse_stream`、Run 队列）。
5. **Worker 未启动**：Redis 不可用时文档异步处理、Run 流式服务会静默降级，启动日志有 `Warn` 提示。

## 8. 常见问题

| 现象 | 排查方向 |
| --- | --- |
| 前端 404 | 确认走 `/api/v1` 前缀（base.js 自动归一化）；确认路由已注册（router.go） |
| 模型连接 404 / 非 JSON 错误 | Base URL 是否填了完整 endpoint，见「模型接入约定」 |
| 文档一直 pending | Redis 是否可用；DocumentWorker 是否启动（启动日志）；查看 processing-jobs 详情 |
| 对话无工具调用过程 | 检查 Trace 中 Tool Span 是否记录；Agent 配置是否启用对应工具/MCP/Skill |
| Trace 查询慢 | 索引是否生效；`trace.retention_days` 是否过大 |

## 9. 相关文档

- `docs/ARCHITECTURE.md` — 架构设计、模块职责、数据流
- `docs/API.md` — 接口清单与认证矩阵
- `docs/数据库设计.md` — 数据表结构
- `docs/RAG系统设计梳理文档.md` — RAG 检索/索引链路
- `docs/model_integration_guide.md` — 模型接入详细指南
