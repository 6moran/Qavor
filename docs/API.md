# API 文档

> 本文档基于代码库 `internal/api/v1/` 下的实际路由编写，与实现保持同步。
> 最后核对日期：2026-08-20

## 基础信息

- **Base URL**: `http://localhost:8080`（端口由 `configs/config.yaml` 的 `app.port` 决定）
- **API 版本**: v1（统一前缀 `/api/v1`）
- **Content-Type**: `application/json`

> 前端注意：`frontend/src/apis/base.js` 的 `normalizeApiUrl` 会把 `/api/xxx` 自动重写为 `/api/v1/xxx`，已是 `/api/v1` 或非 `/api` 开头则原样返回。

## 统一响应格式

所有 API 返回统一格式：

```json
{
  "code": 0,
  "message": "成功",
  "data": {}
}
```

- `code`: 错误码，0 表示成功
- `message`: 响应消息
- `data`: 响应数据（可为 null）

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未授权（未登录 / Token 无效 / 已登出） |
| 403 | 禁止访问 |
| 404 | 资源不存在 |
| 405 | 方法不允许 |
| 408 | 请求超时 |
| 409 | 资源冲突 |
| 500 | 服务器内部错误 |
| 501 | 功能未实现 |
| 503 | 服务不可用 |
| 1001-1006 | 用户/凭据相关业务错误（保留） |
| 2001-2003 | 参数错误：参数错误 / 缺少必要参数 / 参数格式错误 |
| 3001-3003 | 资源错误：资源不存在 / 已存在 / 已锁定 |
| 4001-4006 | LLM 错误：内部错误 / 配置错误 / 请求失败 / 响应无效 / 超时 / 超出 token 限制 |
| 5001-5004 | 模型提供商错误：不存在 / 已存在 / 已禁用 / API Key 未配置 |
| 40001-40003 | 会话错误：不存在 / 无权访问 / 状态无效 |
| 40011-40013 | 消息错误：不存在 / 无权访问 / 角色无效 |
| 6001-6014 | SSE 流式服务错误（见 `pkg/errors/code.go`） |

## 认证方式

单实例管理员登录（单用户模式），JWT Bearer Token：

```
Authorization: Bearer {token}
```

**登录**：`POST /api/v1/auth/login`，使用 `configs/config.yaml` 中的 `auth.admin_username` / `auth.admin_password`。

> 说明：当前为单 Token 机制（区别于旧文档的双 Token）。Token 默认有效期 2 小时（`config.yaml` → `jwt.expire_hours`），登出后加入 Redis 黑名单立即失效。

### 认证矩阵（路由级中间件，与代码一致）

| 模块路由 | 是否挂 `Auth()` 中间件 |
| --- | --- |
| `/auth/login`、`/health` | 否（公开） |
| `/models`（含 providers / remote-models / `/:id`） | 否 |
| `/system/ocr/*`、`/system/tools` | 否 |
| `/auth/logout` | 否（控制器内部自行校验 Token） |
| `/models/test` | 是 |
| 其余全部（knowledge / agent / chat / conversations / rag / system/config / skills / mcp / sse / workspace / traces / dashboard / evaluation 等） | 是 |

> 即使路由未强制认证，前端所有业务请求也会携带 Token；未认证仅影响服务端校验强度。

---

## 健康检查

### 检查服务状态

```http
GET /api/v1/health
```

**响应示例**:
```json
{
  "status": "ok",
  "message": "Qavor API is running"
}
```

---

## 认证

### 管理员登录

```http
POST /api/v1/auth/login
```

**请求参数**:
```json
{
  "username": "12345678",
  "password": "12345678"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 管理员用户名（config.yaml `auth.admin_username`） |
| password | string | 是 | 管理员密码（config.yaml `auth.admin_password`） |

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

### 管理员登出

```http
POST /api/v1/auth/logout
Authorization: Bearer {token}
```

将当前 Token 加入 Redis 黑名单，立即失效。

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

---

## 模型管理

### 创建模型

```http
POST /api/v1/models
```

**请求参数**（示意）:
```json
{
  "name": "gpt-4o",
  "provider": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-xxx",
  "type": "chat",
  "capabilities": ["chat", "tools"],
  "context_window": 128000
}
```

**响应示例**:
```json
{ "code": 0, "message": "成功", "data": null }
```

### 模型列表

```http
GET /api/v1/models
```

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "list": [
      {
        "id": 1,
        "name": "gpt-4o",
        "provider": "openai",
        "base_url": "https://api.openai.com/v1",
        "type": "chat",
        "capabilities": ["chat", "tools"],
        "context_window": 128000,
        "created_at": "2026-08-01T00:00:00Z"
      }
    ],
    "total": 1
  }
}
```

### 模型供应商列表

```http
GET /api/v1/models/providers
```

返回支持的供应商选项（名称、说明、是否内置默认 Base URL 等）。

### 获取单个供应商

```http
GET /api/v1/models/providers/:name
```

### 远程拉取模型列表

```http
POST /api/v1/models/remote-models
```

**请求参数**: `{ "base_url": "...", "api_key": "...", "provider": "..." }`

调用供应商的 `/models`（或 `/v1/models`）接口，返回可用模型列表，供前端选择。

### 测试模型连接

```http
POST /api/v1/models/test
Authorization: Bearer {token}
```

**请求参数**: `{ "base_url": "...", "api_key": "...", "model": "..." }`

通过 eino 链路发起一次真实请求校验连接。

### 获取 / 更新 / 删除模型

```http
GET  /api/v1/models/:id
PUT  /api/v1/models/:id
DELETE /api/v1/models/:id
```

---

## 系统配置

### 全局 RAG 设置

```http
GET /api/v1/system/rag-settings
PUT /api/v1/system/rag-settings
```

读取 / 更新全局 RAG 算法默认值（TopK、相似度阈值、RRF 权重、Rerank 开关等）。

### 系统配置（KV）

```http
GET  /api/v1/system/config
POST /api/v1/system/config
POST /api/v1/system/config/update        # 批量更新
GET  /api/v1/system/config/options       # 可配置项列表（default_model / fast_model / embed_model / rerank 等）
PUT  /api/v1/system/config/options/:key  # 更新单个配置项
```

配置存储于 `system_settings` 表（key-value）。

### OCR 引擎（公开）

```http
GET /api/v1/system/ocr/options
GET /api/v1/system/ocr/health
```

`options` 返回可用 OCR 引擎列表与默认引擎（`rapid_ocr` 本地 / `api_ocr` 通用 API）。

### 工具列表（公开）

```http
GET /api/v1/system/tools
GET /api/v1/system/tools/options
```

返回系统内置工具（query_kb、web_search、calculator 等）注册信息。

### Skill 管理

```http
GET    /api/v1/system/skills                    # 列表
GET    /api/v1/system/skills/options
POST   /api/v1/system/skills                    # 创建
POST   /api/v1/system/skills/batch              # 批量创建
POST   /api/v1/system/skills/delete-batch       # 批量删除
POST   /api/v1/system/skills/import             # 导入（zip/文件）
POST   /api/v1/system/skills/import/prepare     # 导入前预检
GET    /api/v1/system/skills/builtin            # 内置 Skill 列表
POST   /api/v1/system/skills/builtin/sync       # 同步内置 Skill
GET    /api/v1/system/skills/:slug              # 详情
PUT    /api/v1/system/skills/:slug              # 更新
DELETE /api/v1/system/skills/:slug              # 删除
GET    /api/v1/system/skills/:slug/export       # 导出
GET    /api/v1/system/skills/:slug/tree         # 文件树
GET    /api/v1/system/skills/:slug/file         # 读取文件
PUT    /api/v1/system/skills/:slug/file         # 更新文件
DELETE /api/v1/system/skills/:slug/file         # 删除文件
PUT    /api/v1/system/skills/:slug/enabled      # 启用/停用
```

### 远程 Skill 拉取

```http
POST /api/v1/skills/remote/list        # 从远程仓库（GitHub）搜索 Skill
POST /api/v1/skills/remote/prepare     # 拉取前预检
```

---

## 知识库

### 分块预设

```http
GET /api/v1/knowledge/chunk-presets
```

返回可选分块策略（普通 / 层级 / FAQ）与参数预设。

### 知识库 CRUD

```http
GET    /api/v1/knowledge/databases
POST   /api/v1/knowledge/databases
GET    /api/v1/knowledge/databases/:kb_id
PUT    /api/v1/knowledge/databases/:kb_id
DELETE /api/v1/knowledge/databases/:kb_id
```

创建/更新时需绑定：名称、描述、Embedding 模型 ID、Rerank 模型 ID（可选）、分块参数。

### 检索测试与查询参数

```http
POST /api/v1/knowledge/databases/:kb_id/query-test      # 检索测试
GET  /api/v1/knowledge/databases/:kb_id/query-params    # 读取查询参数
PUT  /api/v1/knowledge/databases/:kb_id/query-params    # 更新查询参数
```

### 示例问题

```http
GET  /api/v1/knowledge/databases/:kb_id/sample-questions
POST /api/v1/knowledge/databases/:kb_id/sample-questions   # LLM 生成示例问题
```

### AI 生成描述

```http
POST /api/v1/knowledge/generate-description
```

根据知识库内容（或用户输入）由 LLM 生成/润色知识库描述。

---

## 知识文件

### 上传文件

```http
POST /api/v1/knowledge/files/upload
Content-Type: multipart/form-data
```

| 字段 | 类型 | 说明 |
|------|------|------|
| file | file | 待解析文档（PDF / Word / 图片等） |
| kb_id | int | 目标知识库 ID |
| mode | string | upload（上传）/ folder（从服务器目录选择，可选） |

文件上传至 MinIO，随后进入异步解析队列（Redis Stream），处理进度通过 `/knowledge/processing-jobs` 查询。

### 文件夹

```http
POST /api/v1/knowledge/databases/:kb_id/folders
```

### 文档列表 / 搜索

```http
GET /api/v1/knowledge/databases/:kb_id/documents        # 列表（分页 + 状态筛选）
GET /api/v1/knowledge/databases/:kb_id/documents/search # 搜索
```

### 文档操作

```http
DELETE /api/v1/knowledge/databases/:kb_id/documents/batch                 # 批量删除
POST   /api/v1/knowledge/databases/:kb_id/documents/:doc_id/parse         # 重新解析
POST   /api/v1/knowledge/databases/:kb_id/documents/:doc_id/index         # 单文档入库（分块+向量化）
POST   /api/v1/knowledge/databases/:kb_id/documents/index                 # 批量入库
POST   /api/v1/knowledge/databases/:kb_id/documents/index-pending         # 入库所有待处理文档
GET    /api/v1/knowledge/databases/:kb_id/documents/:doc_id               # 详情
GET    /api/v1/knowledge/databases/:kb_id/documents/:doc_id/content       # 预览（解析文本）
GET    /api/v1/knowledge/databases/:kb_id/documents/:doc_id/download      # 下载原文件
DELETE /api/v1/knowledge/databases/:kb_id/documents/:doc_id               # 删除
```

---

## 文档处理任务

```http
GET  /api/v1/knowledge/processing-jobs            # 任务列表（状态、进度）
GET  /api/v1/knowledge/processing-jobs/:job_id    # 任务详情
POST /api/v1/knowledge/processing-jobs/:job_id/retry  # 重试失败任务
```

任务生命周期：`pending → processing → succeeded / failed`（见 `document_processing_job` 实体）。

---

## 知识导图

```http
GET  /api/v1/knowledge/mindmap/databases                        # 有导图的知识库
GET  /api/v1/knowledge/databases/:kb_id/mindmap/files           # 文件列表
GET  /api/v1/knowledge/databases/:kb_id/mindmap                 # 获取导图
GET  /api/v1/knowledge/databases/:kb_id/mindmap/diff            # 变更 diff
POST /api/v1/knowledge/databases/:kb_id/mindmap/generate        # LLM 生成导图
```

---

## RAG 问答

### 快速回答

```http
POST /api/v1/rag/answer
```

**请求参数**（示意）:
```json
{
  "kb_id": 1,
  "question": "什么是退款政策？"
}
```

走完整 RAG 链路：混合检索（向量 + 关键词 + RRF + Rerank）→ 阈值过滤 → Prompt → LLM 生成。

---

## Agent

### Agent CRUD

```http
POST   /api/v1/agent                 # 创建
GET    /api/v1/agent/list            # 列表
GET    /api/v1/agent/default         # 默认 Agent
GET    /api/v1/agent/:slug           # 详情
PUT    /api/v1/agent/:slug           # 更新
DELETE /api/v1/agent/:slug           # 删除
POST   /api/v1/agent/:slug/default   # 设为默认
```

Agent 配置包含：模型绑定（model_id）、System Prompt、工具/MCP/Skill 白名单、温度等。

### Run 流式执行

```http
POST /api/v1/agent/runs                       # 创建 Run，返回 SSE 流（携带 resume 参数时断线重连）
GET  /api/v1/agent/runs/:runId                # 查询 Run 状态
POST /api/v1/agent/runs/:runId/cancel         # 取消 Run
```

Run 通过 Redis Stream 队列交给 RunWorker 异步执行，进度经 Redis Pub/Sub → SSE 实时推送。

### 请求（排队）控制

```http
GET  /api/v1/agent/requests/:requestId        # 请求详情
POST /api/v1/agent/requests/:requestId/cancel # 取消排队请求
POST /api/v1/agent/requests/:requestId/steer  # 引导请求（注入提示）
```

### 线程队列

```http
GET  /api/v1/agent/thread/:threadId/requests              # 线程排队请求列表
POST /api/v1/agent/thread/:threadId/requests/continue     # 继续暂停的队列
GET  /api/v1/agent/thread/:threadId/agent-state           # Agent 状态（断点）
```

---

## 对话

```http
POST /api/v1/chat            # 普通对话（非流式，一次返回）
POST /api/v1/chat/call       # 同 /chat（别名）
POST /api/v1/chat/stream     # 流式对话（SSE，含工具调用过程事件）
```

**请求参数**（示意）:
```json
{
  "conversation_id": 1,
  "content": "帮我查一下退款政策",
  "agent_slug": "assistant"
}
```

---

## 会话与消息

### 会话

```http
POST   /api/v1/conversations
GET    /api/v1/conversations
GET    /api/v1/conversations/:id
PUT    /api/v1/conversations/:id
DELETE /api/v1/conversations/:id
PUT    /api/v1/conversations/:id/close       # 关闭会话
PUT    /api/v1/conversations/:id/archive     # 归档
POST   /api/v1/conversations/:id/clear-context  # 清空上下文（含短期记忆）
```

### 消息

```http
POST   /api/v1/conversations/:id/messages
GET    /api/v1/conversations/:id/messages
GET    /api/v1/conversations/:id/messages/latest
GET    /api/v1/conversations/:id/messages/:msg_id
PUT    /api/v1/conversations/:id/messages/:msg_id
DELETE /api/v1/conversations/:id/messages/:msg_id
```

> 路由说明：`:id` 为会话 ID，`:msg_id` 为消息 ID。消息路由当前未挂 `Auth()` 中间件（与代码一致）。

---

## SSE 服务

```http
GET /api/v1/sse/connect     # 建立 SSE 连接（EventSource）
GET /api/v1/sse/info        # 获取当前连接信息
```

SSE 事件包含心跳（heartbeat）与业务事件（任务状态、工具调用、增量内容等）。

---

## MCP Server 管理

```http
POST   /api/v1/mcp                       # 创建
POST   /api/v1/mcp/test                  # 配置测试
GET    /api/v1/mcp/list                  # 列表
GET    /api/v1/mcp/:name                 # 详情
PUT    /api/v1/mcp/:name                 # 更新
DELETE /api/v1/mcp/:name                 # 删除
POST   /api/v1/mcp/:name/enable          # 启用
POST   /api/v1/mcp/:name/disable         # 停用
POST   /api/v1/mcp/:name/test            # 测试连接
GET    /api/v1/mcp/:name/tools           # 工具列表
POST   /api/v1/mcp/:name/tools/refresh   # 刷新工具
PUT    /api/v1/mcp/:name/tools/:toolName/toggle  # 启停单个工具
```

---

## 工作区

```http
GET    /api/v1/workspace/tree            # 目录树
GET    /api/v1/workspace/file            # 读取文件（?path=...）
PUT    /api/v1/workspace/file            # 保存文件
DELETE /api/v1/workspace/file            # 删除文件
POST   /api/v1/workspace/directory       # 创建目录
POST   /api/v1/workspace/upload          # 上传附件
```

工作区根目录：`data/workspaces`（`config.yaml` → `agent.workspace_root`）。

---

## 链路追踪

```http
GET /api/v1/traces                            # Trace 列表
GET /api/v1/traces/:trace_id                  # Trace 详情（含 spans）
GET /api/v1/traces/:trace_id/spans/:span_id   # 单个 Span 详情
GET /api/v1/runs/:run_id/trace                # 通过 run_id 反向定位 trace_id
```

**Trace 列表 Query 参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| keyword | string | 按问题关键词模糊搜索 |
| agent_slug | string | 按 Agent 筛选 |
| conversation_id | int | 按会话 ID 筛选 |
| status | string | running / success / failed / cancelled / timeout |
| source | string | sync（同步）/ stream（流式）/ run（异步 Run） |
| from / to | string | 时间范围（RFC3339） |
| page / page_size | int | 分页，默认 page=1、page_size=20（最大 100） |

**Trace 详情响应**（示意）:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "trace": {
      "trace_id": "3f0c...",
      "source": "stream",
      "status": "success",
      "query": "什么是退款政策？",
      "duration_ms": 2340,
      "total_tokens": 356
    },
    "spans": [
      {
        "span_id": "a1b2...",
        "parent_span_id": "",
        "kind": "llm",
        "name": "gpt-4o",
        "status": "success",
        "started_at": "2026-08-08T10:00:00Z",
        "ended_at": "2026-08-08T10:00:02Z",
        "duration_ms": 2100,
        "input_summary": "用户问题摘要",
        "output_summary": "模型回复摘要",
        "tokens_in": 200,
        "tokens_out": 156,
        "reasoning_tokens": 0,
        "error_message": ""
      }
    ]
  }
}
```

- `kind` 取值：`llm`（模型调用）/ `tool`（工具调用）/ `retriever`（知识检索）/ `agent`（Agent 节点）
- spans 按 `started_at` 升序平铺返回，前端按 `parent_span_id` 组装层级树
- 数据保留天数、超时标记由 `config.yaml` 的 `trace` 段配置，janitor 定期清理

---

## 仪表盘

```http
GET /api/v1/dashboard/stats/calls/timeseries
```

返回调用量时序统计（按时间聚合）。

---

## RAG 评估

### 数据集（基准）管理

```http
POST   /api/v1/evaluation/databases/:kb_id/datasets/upload    # 上传数据集
GET    /api/v1/evaluation/databases/:kb_id/datasets           # 列表
GET    /api/v1/evaluation/databases/:kb_id/datasets/:dataset_id
POST   /api/v1/evaluation/databases/:kb_id/datasets/generate  # LLM 生成数据集
POST   /api/v1/evaluation/databases/:kb_id/datasets/:dataset_id/resume  # 恢复生成
DELETE /api/v1/evaluation/datasets/:dataset_id
GET    /api/v1/evaluation/datasets/:dataset_id/download
```

### 评估运行

```http
POST   /api/v1/evaluation/databases/:kb_id/runs          # 发起评估
GET    /api/v1/evaluation/databases/:kb_id/runs          # 运行列表
GET    /api/v1/evaluation/databases/:kb_id/runs/:run_id  # 运行结果
DELETE /api/v1/evaluation/databases/:kb_id/runs/:run_id
```

评估指标：P@10、R@10、MRR、NDCG@10、MAP@10（`internal/service/evaluation_metrics.go`）。

---

## 使用示例

### cURL

```bash
# 登录获取 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r '.data.token')

# 知识库列表
curl http://localhost:8080/api/v1/knowledge/databases \
  -H "Authorization: Bearer $TOKEN"

# 模型列表
curl http://localhost:8080/api/v1/models

# 快速回答（RAG）
curl -X POST http://localhost:8080/api/v1/rag/answer \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"kb_id":1,"question":"什么是退款政策？"}'

# Trace 列表
curl "http://localhost:8080/api/v1/traces?page=1&page_size=20" \
  -H "Authorization: Bearer $TOKEN"
```

### JavaScript (Fetch)

```javascript
// 登录
const res = await fetch('/api/v1/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ username: 'admin', password: 'admin' }),
});
const { data } = await res.json();
const token = data.token;

// 携带 Token 请求
const kbs = await fetch('/api/v1/knowledge/databases', {
  headers: { 'Authorization': `Bearer ${token}` },
});
const kbData = await kbs.json();
console.log(kbData.data.list);
```

---

## 注意事项

1. **单管理员认证**：无用户表，登录凭据来自 `config.yaml` 的 `auth.admin_username` / `auth.admin_password`。
2. **单 Token 机制**：`access token` 默认有效期 2 小时（`jwt.expire_hours`）；登出后 Token 加入 Redis 黑名单立即失效，无需 refresh token。
3. **前端路径归一化**：`/api/xxx` 会被 `normalizeApiUrl` 重写为 `/api/v1/xxx`，调用时按 `/api/...` 书写即可。
4. **异步处理**：文档上传后解析、入库、Agent Run 均走异步队列，通过 processing-jobs / runs 接口轮询状态，或通过 SSE 接收实时事件。
5. **错误处理**：按 `code` 字段判断；HTTP 状态码仅反映传输层结果。
6. **时区**：时间字段使用 ISO 8601 / RFC3339。
