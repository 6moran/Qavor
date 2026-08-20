# Qavor 架构设计文档

> 本文档基于当前代码库（`internal/`、`pkg/`、`frontend/`）实际实现编写，与代码保持同步。
> 最后核对日期：2026-08-20

## 1. 概述

Qavor 是面向开发者的 **AI Agent 构建、扩展、运行与观测平台**：将模型接入、Agent 编排、知识库检索（RAG）、工具扩展（MCP / Skill / 内置工具）、异步执行与链路追踪整合在同一个全栈项目中。

- **后端**：Go + Gin（HTTP）+ CloudWeGo Eino（Agent 运行时 / LLM 调用 / Callback 追踪）
- **前端**：Vue 3 + Vite + ant-design-vue
- **数据层**：PostgreSQL（pgvector + pg_trgm）、Redis（Streams / Token 黑名单 / 记忆）、MinIO（对象存储）
- **解析服务**：Python 子进程（`pkg/documentparser/python/parse_document.py`），OCR 支持本地 RapidOCR 与通用 OCR API 两种引擎
- **认证**：单实例管理员登录（单用户模式，无用户表），JWT Bearer Token + Redis 黑名单登出

## 2. 技术栈

| 领域 | 选型 | 用途 |
| --- | --- | --- |
| HTTP 框架 | Gin | 路由、中间件、参数绑定 |
| Agent 编排 | CloudWeGo Eino（`adk` + `callbacks`） | Agent 运行时、LLM 客户端、全局回调采集 |
| ORM | GORM | PostgreSQL 数据访问、AutoMigrate |
| 向量检索 | pgvector + pg_trgm | 向量召回、关键词召回（ILIKE / 相似度） |
| 队列 | Redis Streams | 文档解析队列、Agent Run 请求队列 |
| 事件总线 | Redis Pub/Sub（`internal/eventbus`） | Run 执行进度 → SSE 推送 |
| 对象存储 | MinIO | 知识库文件、图片、工作区附件 |
| 文档解析 | Python 子进程 + RapidOCR / OCR API | PDF/Word/图片等解析为文本 |
| 日志 | Zap + Lumberjack | 结构化日志、文件轮转 |
| 配置 | Viper | `configs/config.yaml` + 环境变量覆盖 |
| 认证 | 自研 JWT（`pkg/jwt`） | 登录签发、中间件校验、黑名单 |

## 3. 目录结构

```
Qavor/
├── cmd/server/main.go        # 入口（约 30 行，仅装配与启动）
├── internal/
│   ├── app/                  # 应用装配：配置/日志/数据库/依赖/路由/服务器
│   ├── api/                  # Gin 路由
│   │   ├── router.go         # 全局中间件 + 各模块路由注册
│   │   └── v1/<module>/      # 每个业务模块一个目录：controller + routes
│   ├── service/              # 业务逻辑（接口 + 实现，依赖注入）
│   ├── repository/           # 数据访问（接口 + 实现）
│   ├── model/
│   │   ├── entity/           # GORM 实体（对应数据库表）
│   │   └── dto/              # request / response 结构
│   ├── middleware/           # Recovery / Logger / CORS / Auth
│   ├── agent/                # Agent 运行时（eino adk）：MCP、Skill、工具、审批、断点
│   ├── rag/                  # RAG 核心：分块、索引、混合检索、重排、回答链路
│   ├── llm/                  # LLM 客户端抽象（OpenAI 兼容协议）
│   ├── embedding/            # Embedding 客户端抽象
│   ├── ingestion/            # 文档解析编排（Python 解析器 + 图片上传）
│   ├── memory/               # 短期记忆（Redis）+ 长期记忆（pgvector）
│   ├── context/              # 对话上下文管理（消息 + 短期 + 长期记忆）
│   ├── tool/                 # 工具注册表 + 内置工具（query_kb 等）
│   ├── skill/                # Skill 加载、管理、远程拉取
│   ├── mcp/                  # MCP Server 管理（stdio/SSE）
│   ├── sse/                  # SSE 连接管理、心跳、推送
│   ├── run/                  # Run 异步执行：请求队列、Worker、事件发布
│   ├── queue/                # 文档解析队列（Redis Stream）
│   ├── eventbus/             # Redis Pub/Sub 发布订阅
│   ├── trace/                # 链路追踪：Tracer / Writer / Handler / Janitor
│   ├── worker/               # 文档处理 Worker（消费解析队列）
│   └── store/                # MCP Server 配置的本地文件存储
├── pkg/
│   ├── config/               # Viper 配置加载与校验
│   ├── logger/               # Zap 初始化
│   ├── database/             # postgres.go / redis.go（连接池）
│   ├── minio/                # MinIO 客户端
│   ├── jwt/                  # Token 生成/解析
│   ├── cache/                # Redis 黑名单等
│   ├── errors/               # 错误码与业务错误
│   ├── response/             # 统一响应封装
│   ├── validator/            # 参数校验错误翻译
│   ├── crypto/               # bcrypt 等
│   ├── documentparser/       # Python 解析脚本与调用封装
│   └── llm/                  # LLM 客户端工厂（OpenAI 兼容）
├── frontend/                 # Vue 3 前端（src/apis 下按模块封装 API）
├── configs/config.yaml       # 配置文件
├── scripts/                  # build.sh / migrate.sql 等
└── Makefile                  # 构建、运行、测试、迁移等目标
```

## 4. 分层架构

```
┌────────────────────────────────────────────────────────────┐
│  HTTP Layer（internal/api + internal/middleware）           │  Gin 路由 / Controller / 认证
├────────────────────────────────────────────────────────────┤
│  Service Layer（internal/service）                          │  业务逻辑 / 事务 / 编排
├────────────────────────────────────────────────────────────┤
│  Domain Layer（internal/rag、agent、memory、run、trace...）  │  领域能力（可被多个 Service 复用）
├────────────────────────────────────────────────────────────┤
│  Repository Layer（internal/repository）                    │  数据访问（GORM / SQL）
├────────────────────────────────────────────────────────────┤
│  Storage（PostgreSQL+pgvector / Redis / MinIO）             │  持久化
└────────────────────────────────────────────────────────────┘
```

### 4.1 分层职责

| 层 | 位置 | 职责 |
| --- | --- | --- |
| API 层 | `internal/api/v1/<module>/` | 参数绑定与校验、调用 Service、统一响应（`pkg/response`） |
| Service 层 | `internal/service/` | 业务规则、跨模块编排、事务边界、接口定义（供 Controller 与测试 Mock） |
| Domain 层 | `internal/{rag,agent,memory,run,trace,...}` | 领域能力：检索、Agent 循环、记忆、队列执行、追踪采集 |
| Repository 层 | `internal/repository/` | 数据访问，接口 + GORM 实现；SQL 集中在实现内 |
| 基础设施 | `pkg/` | 配置、日志、数据库、存储、JWT、错误、响应等无业务语义的通用能力 |

### 4.2 请求流程

```
HTTP Request
    ↓
[Middleware] Recovery → Logger → Trace 中间件（透传 trace_id）→ CORS → Auth（JWT + 黑名单）
    ↓
[Controller] 参数绑定（ShouldBindJSON + validator）→ 调用 Service
    ↓
[Service]    业务编排 → Repository / Domain 模块
    ↓
[Repository] PostgreSQL / Redis / MinIO
    ↓
[Response]   统一 {code, message, data} JSON
```

> 说明：`Auth` 中间件按模块挂载，并非全局。`/models`、`/system/ocr/*`、`/system/tools`、`/auth/login`、`/health` 等路由未挂 `Auth()`（与当前实现一致，详见 `docs/API.md` 认证矩阵）。

### 4.3 依赖注入

所有层通过构造函数注入依赖，统一在 `internal/app/app.go` 的 `initDependencies()` 中装配：

```go
knowledgeBaseRepo := repository.NewKnowledgeBaseRepository(a.postgresDB)
knowledgeBaseSvc := service.NewKnowledgeBaseService(knowledgeBaseRepo, modelRepo, knowledgeFileRepo, storage, agentRepo)
// ...
a.router = api.NewRouter(authSvc, knowledgeBaseSvc, ...)
```

## 5. 核心业务模块

### 5.1 模型管理（internal/service/model_*）

- 模型数据存 `models` 表（供应商、Base URL、API Key、能力标记、上下文窗口等）
- 支持供应商列表、远程拉取模型列表（`/models/remote-models`）、连接测试（`/models/test`）
- **Base URL 约定为 API 根地址**（如 `https://api.siliconflow.cn/v1`），内部拼接 `/models`、`/v1/models`、`/chat/completions` 等路径
- 知识库 / Agent / 记忆模块通过 `modelID` 动态解析模型（`modelSvc.CreateLLMClient(ctx, modelID)`），模型按需绑定

### 5.2 知识库与 RAG（internal/rag、internal/ingestion）

**索引链路**：上传文件 → MinIO → Redis Stream 解析队列 → DocumentWorker → Python 解析（OCR）→ 分块（默认 800 tokens / overlap 100，支持层级/FAQ 分块预设）→ Embedding → 写入 pgvector（`knowledge_chunks`）。

**问答链路（Answer Graph）**：
```
query
  ↓ 查询向量化（按知识库绑定 Embedding 模型）
  ↓ 混合检索（HybridRetriever）
  ├── 向量召回（pgvector，按知识库分组）
  ├── 关键词召回（pg_trgm / ILIKE）
  ├── RRF 融合（Reciprocal Rank Fusion）
  └── Rerank 重排（HTTP reranker，按知识库绑定）
  ↓ 相似度阈值过滤（score_threshold，默认 0.3）
  ↓ Prompt 组装（引用片段 + 问题）
  ↓ LLM 生成回答
```

检索参数（TopK、阈值、RRF 权重等）既可在知识库查询参数中配置，也可走全局 `rag-settings`。

### 5.3 Agent 运行时（internal/agent，基于 eino adk）

- `AgentManager` 组装 MCP Manager、Tool Registry、Skills Middleware、Agent Service、运行时
- `AgentRuntime`：本地文件系统安全策略（`internal/agent/localfs/security`）、工作区根目录、Shell 超时、后台任务管理器、CheckPoint Store（审批断点，Redis 持久化）
- 支持子智能体（Subagent）、审批中断（Approval）、工具错误恢复中间件、AskUser 交互

### 5.4 工具系统（internal/tool）

- `tool.Registry` 统一注册表；内置工具：`query_kb`（知识库检索）、`web_search`（联网搜索，Tavily/Brave）、`calculator`、`current_time`、`ask_user` 等
- MCP Server 的工具经 `ToolVectorizer` 汇总，支持按名称启停（toggle）
- Agent 在对话中按需调用工具，工具调用过程通过 Trace 记录

### 5.5 Skill 系统（internal/skill）

- 从 `qavor/skills` 目录加载 Skill（SKILL.md + 脚本），支持依赖校验、动态激活
- API：`/system/skills` 管理（CRUD、导入导出、文件树）、`/skills/remote/*` 远程仓库拉取（GitHub）
- 运行时通过 `SkillsMiddleware` 将 Skill 暴露为工具供 Agent 调用

### 5.6 MCP 系统（internal/mcp）

- 管理 MCP Server（stdio / SSE 两种传输），配置存本地文件（`qavor/` 数据目录）+ 数据库
- 启动时按 Agent 配置白名单预热；支持测试连接、刷新工具列表、启停工具

### 5.7 记忆系统（internal/memory + internal/context）

- **短期记忆**：Redis 存储 + 消息缓冲（Buffer）+ 会话状态 + LLM 摘要生成，随会话生命周期管理
- **长期记忆**：LLM 从对话中抽取用户画像/偏好/决策/项目事实 → `long_term_memories` 表；P0 阶段全量注入，预留 pgvector 语义检索 Top-K
- **上下文管理器**（`internal/context`）：组合消息历史 + 短期记忆 + 长期记忆，按上下文窗口（MaxTokens 32768 / ReserveTokens 4096）裁剪后注入 System Prompt

### 5.8 对话与 SSE（internal/sse）

- 普通对话：`POST /chat`、`POST /chat/call`
- 流式对话：`POST /chat/stream`（SSE），事件含心跳（heartbeat）与业务事件
- SSE Manager 管理连接、心跳、并发限制（每用户 5 连接）、空闲清理
- 前端通过 EventSource 或 fetch 流式读取

### 5.9 Run 异步执行（internal/run + internal/eventbus）

- `POST /agent/runs`：创建 Run，请求写入 Redis Stream 队列，立即返回 SSE 流（`PostStreamHandler`）
- `RunWorker` 池消费队列 → `AgentExecutor` 执行 Agent（后台任务）→ 事件通过 `eventbus.Publisher` 发布到 Redis Pub/Sub → SSE 推送前端
- 支持：断线重连（resume）、取消（cancel）、排队引导（steer）、线程队列继续（continue）
- 断点信息存 Redis（CheckPoint Store），中断后可恢复

### 5.10 链路追踪（internal/trace）

- **采集**：eino `callbacks.AppendGlobalHandlers` 全局注册 Handler，采集 LLM / Tool / Retriever / Agent 节点调用
- **存储**：Tracer → Writer（异步缓冲批量落库）→ `trace_records` / `trace_spans`（及 Agent 侧 `agent_traces` / `agent_trace_spans`）
- **透传**：Trace 中间件从请求头读取/生成 trace_id，经 Context 传递到 RAG、Worker、后台任务
- **清理**：Trace Janitor 定期将超时 running 标记为 timeout、物理删除过期数据（保留天数、间隔在 `config.yaml` 的 `trace` 段配置）

### 5.11 RAG 评估（internal/service/evaluation_*）

- 数据集（基准）管理：上传 / 生成 / 恢复生成 / 下载
- 评估运行：后台执行器消费任务，指标实现于 `evaluation_metrics.go`（P@10、R@10、MRR、NDCG@10、MAP@10）

### 5.12 其他

- **工作区**（`internal/agent/localfs`）：Agent 工作区文件树、读写、上传，带安全策略
- **仪表盘**：只读统计（调用时序等）
- **知识导图**：基于知识库文件生成思维导图（LLM 生成 + diff 查看）

## 6. 关键数据流

### 6.1 文档解析与入库流水线

```
前端上传（POST /knowledge/files/upload）
  → MinIO 保存原文件，知识文件记录（knowledge_files）
  → Redis Stream 写入解析任务（queue.DocumentQueue）
  → DocumentWorker（internal/worker）消费
      → ingestion.Parser：Python 子进程解析（parse_document.py）
          → RapidOCR / 通用 OCR API（图片/扫描件）
      → rag.DocumentIndexer 分块（chunker / hierarchy / faq）
      → rag.Embedder 向量化（按知识库绑定 Embedding 模型）
      → pgvector 批量入库（knowledge_chunks）
  → 更新处理任务状态（processing_jobs），失败可重试
```

### 6.2 RAG 混合检索问答

```
POST /rag/answer
  → rag_service.Answer → DynamicAnswerEngine（answer_graph）
      → DynamicRetriever（向量）+ KeywordRetriever（关键词）
      → RRF 融合 → DynamicReranker（HTTP 重排）
      → 阈值过滤 → Prompt 组装 → LLM 生成
  → 过程 Span 写入 Trace
```

### 6.3 Agent 流式对话

```
POST /chat/stream（SSE）
  → ChatService
      → ContextManager 组装上下文（历史 + 短期记忆 + 长期记忆）
      → AgentManager 启动 eino Agent 循环（LLM + 工具 + MCP + Skill）
      → 工具调用（query_kb / web_search / MCP / Skill）经 ToolRegistry
      → 结果流式回推 SSE；消息落库；记忆更新
  → eino Callback → Trace（LLM/Tool/Retriever/Agent Span）
```

### 6.4 Run 异步执行

```
POST /agent/runs
  → run.RequestQueue 写入 Redis Stream
  → RunWorker 池消费 → AgentExecutor 执行
      → 进度/工具调用/状态事件 → eventbus.Publisher → Redis Pub/Sub
      → PostStreamHandler 订阅 → SSE 推送给发起连接
  → GET /agent/runs/:runId 查询状态；POST cancel / steer 控制
```

## 7. 后台任务与 Worker 一览

| 任务 | 位置 | 触发方式 | 说明 |
| --- | --- | --- | --- |
| DocumentWorker | `internal/worker` | Redis Stream 消费 | 文档解析 + 分块 + 向量化入库 |
| RunWorker 池 | `internal/run` | Redis Stream 消费 | Agent 异步执行（多 worker 并发） |
| Trace Janitor | `internal/trace` | 定时（janitor_interval） | 超时标记 + 过期清理 |
| RAG 评估执行器 | `internal/service` | 应用启动 + 任务队列 | 数据集生成 / 评估运行 |
| SSE Cleaner | `internal/sse` | 定时（5min） | 清理超时/失效连接 |
| MCP Preheat | `internal/mcp` | 应用启动 | 按 Agent 白名单预热 MCP Server |

## 8. 配置体系

配置源：`configs/config.yaml`（Viper 加载），敏感字段（密码、API Key）支持环境变量覆盖。

| 配置段 | 关键项 | 说明 |
| --- | --- | --- |
| `app` | name / version / mode / port | 服务基础信息 |
| `auth` | admin_username / admin_password | 单管理员登录凭据 |
| `database` | postgres / redis / minio | 连接信息（`auto_migrate` 控制是否自动建表） |
| `jwt` | secret / expire_hours | Token 签发与有效期（默认 2h） |
| `log` | level / filename / max_size... | Zap 日志轮转 |
| `cors` | allow_origins / methods / headers | 跨域 |
| `llm` | model / api_key / base_url / timeout... | 全局默认 LLM 参数 |
| `sse` | max_stream_time / heartbeat_interval / max_concurrent_tasks | SSE 服务 |
| `rag` | chunk_tokens / top_k / score_threshold / embedding... | RAG 算法默认值（模型按知识库绑定） |
| `trace` | enabled / retention_days / timeout_minutes / janitor_interval | 链路追踪 |
| `web_search` | provider / base_url / api_key | 联网搜索工具 |
| `document_parser` | python_path | Python 解析器路径 |
| `document_queue` / `run` / `memory` / `agent` | 队列、Run、记忆、Agent 运行时参数 | 见 config.yaml 注释 |

## 9. 应用装配与启动

`cmd/server/main.go` 仅做三件事：`app.NewApp()` → `Initialize()` → `Run()`。

`internal/app/app.go` 的 `Initialize()` 顺序：

1. **initConfig**：加载配置 + 校验认证配置 + 归一化 workspace_root 绝对路径
2. **initLogger**：Zap 初始化
3. **initDatabase**：PostgreSQL（可选 AutoMigrate）+ Redis（可选）
4. **initMinIO**（可选，失败仅告警）
5. **initDependencies**：Repository → Service → RAG（indexer/retriever/reranker/answer）→ Worker → MCP → Tool/Skill → Agent 运行时 → 记忆/上下文 → SSE → Chat → Run 流式 → Trace → Dashboard → 评估 → Router
6. **initRouter**：设置 Gin 模式
7. **initServer**：创建 `http.Server`

`Run()` 启动评估执行器与 HTTP 服务器，并进入优雅关闭流程（关闭 Worker、Janitor、Trace Writer、MCP、后台任务、数据库连接）。

## 10. 认证与安全

- **登录**：`POST /api/v1/auth/login`，校验 `config.yaml` 的 `auth.admin_username/admin_password`，签发 JWT（默认 2h 有效）
- **中间件**：`middleware.Auth()` 校验 `Authorization: Bearer <token>`，并查 Redis 黑名单（登出后立即失效）
- **登出**：`POST /api/v1/auth/logout`，Token 加入黑名单直至自然过期
- **文件安全**：Agent 工作区与文件操作走本地文件系统安全策略（`internal/agent/localfs/security`）
- **其他**：bcrypt 密码存储（保留）、CORS 限制、参数 validator、GORM 参数化查询防注入

## 11. 前端架构（简述）

- Vue 3 + Vite + ant-design-vue，路由按页面组织（登录、知识库、Agent、对话、追踪、系统设置等）
- API 封装：`frontend/src/apis/<module>_api.js`，统一走 `base.js` 的 `apiRequest`
- **路径归一化**：`base.js` 的 `normalizeApiUrl` 将 `/api/xxx` 重写为 `/api/v1/xxx`（已是 `/api/v1` 或非 `/api` 开头则原样返回）
- 流式对话/任务状态通过 SSE 接收；Trace 详情页按 `parent_span_id` 组装 Span 树

## 12. 相关文档

- `docs/API.md` — 完整接口清单与认证矩阵
- `docs/DEVELOPMENT.md` — 开发指南（新增模块、测试、调试）
- `docs/RAG系统设计梳理文档.md` — RAG 索引/问答链路细节
- `docs/数据库设计.md` — 数据表结构
- `docs/model_integration_guide.md` — 模型接入指南
- `docs/Task-4~7` — 上下文管理、SSE、短期/长期记忆设计文档
- `docs/Agent对话链路追踪设计文档.md`、`docs/后端Span设计文档.md` — 链路追踪设计
