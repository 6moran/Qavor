# Task 5: SSE 流式服务设计（POST 流式 + Redis Stream 持久化）

## 一、概述

本模块为 Qavor AI 助手平台提供 Server-Sent Events (SSE) 流式服务，用于 Agent 长时间运行任务的实时事件推送。

### 1.1 设计方案

采用 **POST 单连接流式 + Redis Stream 持久化** 方案：

- **单连接流式**：前端 `POST /api/agent/runs` 创建 Run 后，**同一连接直接返回 SSE 流**，无需二次 GET 订阅
- **Redis Stream 作为消息总线**：Run Worker 通过 `XADD` 写入事件，POST Handler 通过 `XREAD BLOCK` 读取并推流，二者解耦
- **resume 参数重连**：连接断开后，前端再次 `POST /api/agent/runs` 携带 `resume: { run_id, last_seq }`，从断点续传事件，不创建新 Run
- **心跳保活**：POST Handler 周期性发送 SSE 注释行（`: heartbeat`）保活，不占用事件类型，前端 `EventSource` 自动忽略

> **为何不用单独的 GET 订阅端点**：当前为单管理员体系、无需多端同时订阅同一 Run。POST 单连接模式减少一次往返、简化前端编排；断线续传通过 `resume` 参数在同一 POST 端点完成，Redis Stream 仍保留持久化与续传能力。

### 1.2 核心优势

| 特性 | 说明 |
|------|------|
| **单连接简化** | POST 创建 Run 后直接流式返回，前端无需管理「先 POST 再 GET」两次请求 |
| **生产消费解耦** | Run 执行与 SSE 推送通过 Redis Stream 解耦，Worker 不感知连接状态 |
| **断线续传** | Redis Stream 的 timestamp-seq 作为事件 ID，`resume.last_seq` 携带续传位置 |
| **跨进程恢复** | SSE 连接断开后可用 `resume` 重新 POST，从上次位置继续，事件不丢失 |
| **长任务支持** | Run 执行时间可达数分钟，事件持久化在 Redis Stream 中，不受 HTTP 超时限制 |
| **工具审批中断** | Run 进入 `interrupted` 状态暂停，前端恢复后用 `resume` 从断点续传 |
| **心跳保活** | 周期性 SSE 注释行防止代理/网关因空闲超时关闭连接 |

### 1.3 无用户体系说明

> **当前项目无多用户体系**，SSE Controller 使用固定的 `admin` 标识。所有 Run、Thread、SSE 连接均归属于 `admin`。
> 鉴权仍走 `middleware.Auth()`（JWT），但实际为单一管理员账户。测试时需使用 dev 登录模式（见第九章）。

---

## 二、架构设计

### 2.1 POST 单连接流式通信模式

```
┌──────────┐        ┌──────────────────────────────┐        ┌──────────────┐
│  前端     │        │         后端                 │        │   Redis      │
│ (Vue.js) │        │         (Go)                 │        │   Stream     │
└────┬─────┘        └──────────────┬───────────────┘        └──────┬───────┘
     │                             │                               │
     │  ① POST /api/agent/runs     │                               │
     │  (创建 Run + 建立 SSE 流)    │                               │
     │────────────────────────────>│                               │
     │                             │  ② 创建 AgentRun，入队，        │
     │                             │     写 SSE 响应头并 Flush       │
     │                             │                               │
     │                             │  ③ Run Worker 异步执行 Agent   │
     │                             │     XADD 事件 ──────────────>│
     │                             │     (metadata/chunk/end/...)  │
     │                             │                               │
     │                             │  ④ POST Handler XREAD BLOCK ─>│
     │                             │     (从 0-0 开始)              │
     │                             │<─────────── 事件列表 ─────────│
     │                             │                               │
     │  ⑤ SSE 流式推送（同一连接）   │   ← 周期心跳注释行保活          │
     │  : heartbeat                │                               │
     │<────────────────────────────│                               │
     │  id: <seq>                  │                               │
     │  event: metadata            │                               │
     │  data: {...}                │                               │
     │<────────────────────────────│                               │
     │  ...message chunks...       │                               │
     │  event: end / error         │                               │
     │<────────────────────────────│                               │
     │                             │                               │
     │  ⑥ 收到 end/error，关闭连接  │                               │
     │                             │                               │
```

**断线重连**（同一 POST 端点，携带 `resume`）：

```
前端检测到 SSE 连接异常关闭
     │
     │  POST /api/agent/runs
     │  Body: { "resume": { "run_id": "run-xxx", "last_seq": "1234568-0" } }
     │────────────────────────────>│
     │                             │  校验 Run 存在且未失效，        │
     │                             │  不创建新 Run                   │
     │                             │  XREAD BLOCK 从 last_seq 之后 ─>│
     │                             │<─────────── 剩余事件 ──────────│
     │  续传事件流（跳过已处理）     │                               │
     │<────────────────────────────│                               │
```

### 2.2 核心组件

```
┌─────────────────────────────────────────────────────────┐
│                      后端 (Go)                          │
│                                                         │
│  ┌──────────────┐   ┌──────────────┐  ┌──────────────┐ │
│  │ Run API      │   │ POST Stream  │  │ Run Worker   │ │
│  │ Controller   │   │ Handler      │  │ (Publisher)  │ │
│  │              │   │ (Subscriber) │  │              │ │
│  │ POST /runs   │──>│ XREAD BLOCK  │  │ XADD 事件    │ │
│  │ GET  /runs   │   │ → SSE Writer │  │ Agent.Run()  │ │
│  │ POST /cancel │   │ + 心跳注释行  │  │              │ │
│  └──────┬───────┘   └──────┬───────┘  └──────┬───────┘ │
│         │                  │                  │         │
│         v                  v                  v         │
│  ┌──────────────────────────────────────────────────┐   │
│  │              Redis Stream (消息总线)              │   │
│  │                                                  │   │
│  │  qavor:run:{runId}:events                        │   │
│  │  ├── 1234567-0  metadata {run_id, thread_id,...} │   │
│  │  ├── 1234568-0  message   {chunk: {...}}         │   │
│  │  ├── 1234569-0  message   {chunk: {...}}         │   │
│  │  ├── 1234570-0  message   {items: [...]}        │   │
│  │  └── 1234571-0  end       {status: completed}    │   │
│  │                                                  │   │
│  │  MAXLEN ~ 10000 (近似裁剪)                        │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

> **职责说明**：`Run API Controller` 接收 POST 请求，创建 `AgentRun` 并入队后，将请求转交 `POST Stream Handler`；后者在同一 HTTP 连接上订阅 Redis Stream 并以 SSE 格式推流，同时起一个独立 goroutine 周期发送心跳注释行。`Run Worker` 与 SSE 连接完全解耦，仅通过 Redis Stream 通信。

### 2.3 请求流程

#### 2.3.1 新建 Run（首连接）

1. **创建并建流**：前端 `POST /api/agent/runs`（无 `resume`），后端创建 `AgentRun` 记录（状态 `pending`），入队请求，**立即在同一连接写 SSE 响应头并 Flush**
2. **订阅事件**：POST Handler 通过 `XREAD BLOCK` 从 `0-0` 开始订阅该 Run 的 Redis Stream
3. **异步执行**：Run Worker 从队列取出请求，状态变 `running`，通过 `XADD` 发布 `metadata` 事件
4. **流式推送**：POST Handler 读取到事件后逐条以 SSE 格式推送，并周期发送 `: heartbeat` 注释行保活
5. **事件流**：`metadata` → `message(chunk)` × N → `end`（或 `error`）
6. **连接关闭**：收到终态事件（`end`/`error`）后，POST Handler 关闭连接

#### 2.3.2 断线重连（resume）

1. **检测断开**：前端检测到 SSE 连接异常关闭（`EventSource` onerror 或 fetch 流读取中断）
2. **携带 resume 重连**：前端再次 `POST /api/agent/runs`，Body 含 `resume: { run_id, last_seq }`（`last_seq` 为最后收到的事件 ID）
3. **校验不新建**：后端校验 `run_id` 存在、归属当前 `thread_id`、未过 TTL，**不创建新 Run**
4. **续传推送**：POST Handler 从 `last_seq` 之后（`XREAD` 的 `>`）继续读取并推流，前端按 seq 去重
5. **终态处理**：若 Run 已终态且事件已全部推送，直接补发剩余事件后关闭；若仍在运行，继续订阅至终态

#### 2.3.3 工具审批中断恢复

- Run 因工具审批进入 `interrupted` 时，Worker 发布 `end` 事件（`status: interrupted`，含 `interrupt` 快照），连接关闭
- 前端展示审批 UI，用户决策后通过 POST `resume` 携带 `tool_call_id` + `decision` 恢复执行（具体见 3.5）

---

## 三、前后端接口契约

### 3.1 接口总览

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/api/agent/runs` | 创建异步 Run 任务并**直接返回 SSE 流**；携带 `resume` 时为断线重连 | JWT |
| GET | `/api/agent/runs/:runId` | 获取 Run 状态 | JWT |
| POST | `/api/agent/runs/:runId/cancel` | 取消 Run | JWT |
| GET | `/api/agent/requests/:requestId` | 获取请求详情 | JWT |
| POST | `/api/agent/requests/:requestId/cancel` | 取消排队中的请求 | JWT |
| POST | `/api/agent/requests/:requestId/steer` | 提升为下一条执行的引导请求 | JWT |
| GET | `/api/agent/thread/:threadId/requests` | 列出线程排队请求 | JWT |
| POST | `/api/agent/thread/:threadId/requests/continue` | 继续暂停的线程队列 | JWT |

> **注意**：路由前缀为 `/api`，`middleware.Auth()` 中间件校验 JWT。当前无多用户体系，所有请求归属 `admin`。
>
> **无独立 GET 订阅端点**：SSE 事件流由 `POST /api/agent/runs` 直接返回；断线续传通过同一 POST 携带 `resume` 参数完成，不另设 `GET /runs/:runId/events`。

### 3.2 创建 Run / 断线重连（POST 流式）

**POST** `/api/agent/runs`

该端点承担两种语义，由请求体是否携带 `resume` 区分：
- **新建 Run**：不携带 `resume`（或 `resume` 为 `null`），创建 Run、入队、直接返回 SSE 流
- **断线重连**：携带 `resume: { run_id, last_seq }`，不创建新 Run，从 `last_seq` 续传事件流

#### 3.2.1 新建 Run 请求体

```json
{
  "query": "帮我查询今天的天气",
  "agent_slug": "default",
  "thread_id": "conv-uuid-xxx",
  "meta": {},
  "image_content": null,
  "model_spec": null,
  "tool_approval_mode": null,
  "resume": null,
  "created_by_run_id": null,
  "queue_policy": "enqueue"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 新建时必填 | 用户输入内容（重连时忽略） |
| `agent_slug` | string | 新建时必填 | Agent 标识 |
| `thread_id` | string | 是 | 对话线程 ID（重连时用于校验 Run 归属） |
| `meta` | object | 否 | 附加元数据 |
| `image_content` | string\|null | 否 | 图片内容（多模态） |
| `model_spec` | object\|null | 否 | 模型规格覆盖 |
| `tool_approval_mode` | string\|null | 否 | 工具审批模式：`auto`/`require`/`auto_read` |
| `resume` | object\|null | 否 | 重连参数，见 3.2.2；为 `null` 表示新建 |
| `created_by_run_id` | string\|null | 否 | 父 Run ID（子 Agent 场景） |
| `queue_policy` | string | 否 | 队列策略：`enqueue`（排队）/`steer`（引导）/`cancel_previous` |

#### 3.2.2 断线重连请求体

```json
{
  "thread_id": "conv-uuid-xxx",
  "resume": {
    "run_id": "run-uuid-xxx",
    "last_seq": "1234568-0"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `resume.run_id` | string | 是 | 要重连的 Run ID |
| `resume.last_seq` | string | 是 | 最后收到的事件 ID（Redis Stream seq），从其后续传 |
| `resume.tool_call_id` | string | 否 | 工具审批恢复时携带，指定要恢复的工具调用 |
| `resume.decision` | string | 否 | 工具审批决策：`approved`/`rejected` |

#### 3.2.3 响应

**响应头**（无论是新建还是重连，成功后均为 SSE 流）：
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

**响应体**（SSE 流）：
```
: heartbeat

id: 1234567-0
event: metadata
data: {"event":"metadata","run_id":"run-xxx","thread_id":"conv-xxx","payload":{"run_type":"chat","source":"chat"}}

id: 1234568-0
event: message
data: {"event":"message","run_id":"run-xxx","request_id":"req-xxx","payload":{"chunk":{"type":"token","content":"你好"}}}

id: 1234569-0
event: message
data: {"event":"message","run_id":"run-xxx","payload":{"chunk":{"type":"token","content":"，我是"}}}

id: 1234570-0
event: end
data: {"event":"end","run_id":"run-xxx","payload":{"status":"completed"}}
```

> **首帧约定**：服务端写完响应头后立即 Flush，并可在等待首个事件前先发一行 `: heartbeat` 注释行，让前端尽快确认连接已建立。`run_id` 通过首个 `metadata` 事件的 `data` 返回（而非独立 JSON 响应）。

**错误响应**（重连校验失败时，返回普通 JSON，非 SSE）：
```json
{
  "code": 4004,
  "message": "run not found or expired"
}
```

### 3.3 获取 Run 状态

**GET** `/api/agent/runs/:runId`

**响应**（200）：
```json
{
  "code": 0,
  "data": {
    "run": {
      "id": "run-uuid-xxx",
      "conversation_thread_id": "conv-uuid-xxx",
      "agent_slug": "default",
      "status": "running",
      "request_id": "req-uuid-xxx",
      "run_type": "chat",
      "last_event_id": "1234567-0",
      "started_at": "2026-08-06T10:00:00Z",
      "finished_at": null,
      "created_at": "2026-08-06T10:00:00Z"
    }
  }
}
```

**Run 状态机**：

| 状态 | 说明 | 是否终态 |
|------|------|----------|
| `pending` | 已创建，等待队列调度 | 否 |
| `running` | 正在执行 | 否 |
| `interrupted` | 因工具审批暂停 | 否（可恢复） |
| `completed` | 正常完成 | 是 |
| `failed` | 执行失败 | 是 |
| `cancelled` | 已取消 | 是 |

### 3.4 取消 Run

**POST** `/api/agent/runs/:runId/cancel`

**请求体**：`{}`

**响应**（200）：
```json
{
  "code": 0,
  "data": {
    "run_id": "run-uuid-xxx",
    "status": "cancelled"
  }
}
```

### 3.5 心跳与续传机制

#### 3.5.1 心跳保活

POST 流式连接在等待事件期间（如 Agent 思考、工具执行）可能长时间无数据，易被反向代理/网关因空闲超时关闭。采用 **SSE 注释行** 保活：

```
: heartbeat
```

- **格式**：以 `:` 开头的注释行，后接两个换行符（`\n\n`），符合 SSE 规范
- **不影响事件流**：浏览器 `EventSource` 与 fetch 流式读取均自动忽略注释行，不触发 `onmessage`，不占用 `event` 类型
- **周期**：默认每 15 秒发送一次（可配置 `sse.heartbeat_interval`）
- **触发**：POST Handler 启动独立 goroutine，按 ticker 周期向响应写入 `: heartbeat\n\n` 并 Flush
- **首帧**：响应头写完后可立即发一次心跳，让前端尽快确认连接已建立

#### 3.5.2 续传机制

- **事件 ID 格式**：Redis Stream 的 `{timestamp}-{seq}`，如 `1234567890123-0`，写入 SSE 帧的 `id:` 字段
- **前端记录**：每收到一个事件，前端更新本地快照 `last_seq`（持久化到 `localStorage`）
- **断线重连**：前端再次 `POST /api/agent/runs`，Body 携带 `resume: { run_id, last_seq }`，后端从 `last_seq` 之后（`XREAD` 的 `>`）续传
- **去重**：前端按 `id` 比较已处理 seq，跳过重复事件
- **TTL**：Run 事件流在 Redis 中保留 `retention`（默认 24h），过期后重连返回错误
- **无需 `Last-Event-ID` 头**：续传位置通过请求体 `resume.last_seq` 携带，不依赖 SSE 标准 `Last-Event-ID` 请求头（POST 请求不自动携带该头）

#### 3.5.3 查询参数

POST `/api/agent/runs` 支持以下查询参数（可选）：

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `verbose` | bool | false | 是否输出详细事件（调试用，含内部事件） |

### 3.6 请求队列接口

#### 3.6.1 获取请求详情

**GET** `/api/agent/requests/:requestId`

**响应**（200）：
```json
{
  "code": 0,
  "data": {
    "request": {
      "id": "req-uuid-xxx",
      "thread_id": "conv-uuid-xxx",
      "agent_slug": "default",
      "status": "queued",
      "source": "chat",
      "queue_position": 1
    }
  }
}
```

#### 3.6.2 取消排队请求

**POST** `/api/agent/requests/:requestId/cancel`

**响应**（200）：`{ "code": 0, "data": { "request_id": "req-xxx", "status": "cancelled" } }`

#### 3.6.3 引导请求（Steer）

**POST** `/api/agent/requests/:requestId/steer`

将普通排队请求提升为下一条执行的引导请求。

**响应**（200）：`{ "code": 0, "data": { "request_id": "req-xxx", "status": "steered" } }`

#### 3.6.4 列出线程排队请求

**GET** `/api/agent/thread/:threadId/requests?agent_slug=default`

**响应**（200）：
```json
{
  "code": 0,
  "data": {
    "items": [
      { "id": "req-xxx", "status": "queued", "queue_position": 0, "source": "chat" }
    ]
  }
}
```

#### 3.6.5 继续暂停的线程队列

**POST** `/api/agent/thread/:threadId/requests/continue?agent_slug=default`

手动继续 `failed`/`cancelled` 后暂停的线程队列。

**响应**（200）：`{ "code": 0, "data": { "resumed": true } }`

---

## 四、SSE 事件协议

### 4.1 事件类型

事件类型对应 SSE 的 `event:` 字段，共 4 种：

| event 类型 | 说明 | 触发时机 |
|-----------|------|----------|
| `metadata` | Run 元信息 | Run 开始执行时，首发事件 |
| `message` | 内容事件 | Agent 输出 token / 工具调用 / 消息片段 |
| `end` | Run 终态 | Run 完成 / 中断 / 取消 |
| `error` | 错误终态 | Run 执行失败 |

> **心跳注释行**（非事件类型）：除上述 4 种事件外，POST Handler 周期性发送 SSE 注释行 `: heartbeat` 保活。注释行以 `:` 开头，**无 `id`/`event`/`data` 字段**，不进入事件流，前端自动忽略。详见 3.5.1。

### 4.2 事件数据结构

所有事件统一信封格式：

```json
{
  "event": "metadata | message | end | error",
  "run_id": "run-uuid-xxx",
  "thread_id": "conv-uuid-xxx",
  "request_id": "req-uuid-xxx",
  "payload": { ... }
}
```

### 4.3 各事件 payload 详解

#### metadata 事件（首发）

```json
{
  "event": "metadata",
  "run_id": "run-xxx",
  "thread_id": "conv-xxx",
  "request_id": "req-xxx",
  "payload": {
    "run_type": "chat",
    "source": "chat",
    "agent_slug": "default"
  }
}
```

#### message 事件 - token 片段（打字机效果）

```json
{
  "event": "message",
  "run_id": "run-xxx",
  "request_id": "req-xxx",
  "payload": {
    "chunk": {
      "type": "token",
      "content": "你好",
      "request_id": "req-xxx",
      "run_id": "run-xxx",
      "thread_id": "conv-xxx"
    }
  }
}
```

#### message 事件 - 工具调用开始

```json
{
  "event": "message",
  "run_id": "run-xxx",
  "payload": {
    "chunk": {
      "type": "tool_call_start",
      "tool_name": "search",
      "tool_call_id": "call-xxx",
      "args": { "q": "天气" }
    }
  }
}
```

#### message 事件 - 工具调用结果

```json
{
  "event": "message",
  "run_id": "run-xxx",
  "payload": {
    "chunk": {
      "type": "tool_call_result",
      "tool_name": "search",
      "tool_call_id": "call-xxx",
      "result": "搜索结果..."
    }
  }
}
```

#### message 事件 - 批量 chunks

```json
{
  "event": "message",
  "run_id": "run-xxx",
  "payload": {
    "items": [
      { "type": "token", "content": "你好" },
      { "type": "token", "content": "，我是" }
    ]
  }
}
```

#### end 事件 - 正常完成

```json
{
  "event": "end",
  "run_id": "run-xxx",
  "payload": {
    "status": "completed"
  }
}
```

#### end 事件 - 工具审批中断

```json
{
  "event": "end",
  "run_id": "run-xxx",
  "payload": {
    "status": "interrupted",
    "interrupt": {
      "tool_call_id": "call-xxx",
      "tool_name": "send_email",
      "args": { "to": "...", "subject": "..." }
    }
  }
}
```

#### end 事件 - 取消

```json
{
  "event": "end",
  "run_id": "run-xxx",
  "payload": {
    "status": "cancelled"
  }
}
```

#### error 事件

```json
{
  "event": "error",
  "run_id": "run-xxx",
  "payload": {
    "error_type": "AGENT_ERROR",
    "message": "LLM 调用超时",
    "retryable": false
  }
}
```

### 4.4 事件 ID 与续传

- **事件 ID 格式**：Redis Stream 的 `{timestamp}-{seq}`，如 `1234567890123-0`
- **SSE 输出**：`id: 1234567890123-0` 写入 SSE 帧头
- **续传方式**：前端再次 `POST /api/agent/runs`，请求体携带 `resume: { run_id, last_seq: "1234567890123-0" }`（不使用 `Last-Event-ID` 请求头，因 POST 不自动携带该头）
- **后端处理**：`XREAD` 从 `last_seq` 之后（`>`）继续读取

---

## 五、后端实现

### 5.1 Redis Stream 事件总线

```go
package eventbus

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// StreamKey 生成 Run 事件流的 Redis Key
func StreamKey(runID string) string {
    return fmt.Sprintf("qavor:run:%s:events", runID)
}

// Event Redis Stream 中存储的事件
type Event struct {
    EventType  string          `json:"event"`      // metadata / message / end / error
    RunID      string          `json:"run_id"`
    ThreadID   string          `json:"thread_id"`
    RequestID  string          `json:"request_id"`
    Payload    json.RawMessage `json:"payload"`
}

// Publisher 事件发布者（Run Worker 使用）
type Publisher struct {
    client *redis.Client
    maxLen int64 // Stream 最大长度（近似裁剪）
}

func NewPublisher(client *redis.Client, maxLen int64) *Publisher {
    if maxLen <= 0 {
        maxLen = 10000
    }
    return &Publisher{client: client, maxLen: maxLen}
}

// Publish 发布一个事件到 Run 的事件流
// 返回 Redis Stream 消息 ID（timestamp-seq），用于 SSE 的 id 字段
func (p *Publisher) Publish(ctx context.Context, runID string, event Event) (string, error) {
    data, err := json.Marshal(event)
    if err != nil {
        return "", fmt.Errorf("marshal event: %w", err)
    }
    msgID, err := p.client.XAdd(ctx, &redis.XAddArgs{
        Stream: StreamKey(runID),
        MaxLen: p.maxLen,
        Approx: true, // 近似裁剪，性能更好
        Values: map[string]any{
            "event": string(data),
        },
    }).Result()
    if err != nil {
        return "", fmt.Errorf("xadd event: %w", err)
    }
    return msgID, nil
}

// Subscriber 事件订阅者（SSE Handler 使用）
type Subscriber struct {
    client    *redis.Client
    blockTime time.Duration // XREAD BLOCK 时长
}

func NewSubscriber(client *redis.Client) *Subscriber {
    return &Subscriber{
        client:    client,
        blockTime: 30 * time.Second, // 每次阻塞最多 30 秒
    }
}

// Read 从指定位置读取事件（用于断线续传）
// afterSeq 格式为 Redis Stream ID，如 "1234567-0"；传 "0-0" 表示从头读取
func (s *Subscriber) Read(ctx context.Context, runID, afterSeq string) ([]StreamEntry, error) {
    if afterSeq == "" {
        afterSeq = "0-0"
    }
    result, err := s.client.XRead(ctx, &redis.XReadArgs{
        Streams: []string{StreamKey(runID), afterSeq},
        Count:   100,
        Block:   s.blockTime,
    }).Result()
    if err == redis.Nil {
        return nil, nil // 超时无新事件
    }
    if err != nil {
        return nil, fmt.Errorf("xread events: %w", err)
    }
    if len(result) == 0 {
        return nil, nil
    }
    entries := make([]StreamEntry, 0, len(result[0].Messages))
    for _, msg := range result[0].Messages {
        var event Event
        if raw, ok := msg.Values["event"].(string); ok {
            _ = json.Unmarshal([]byte(raw), &event)
        }
        entries = append(entries, StreamEntry{
            ID:    msg.ID,
            Event: event,
        })
    }
    return entries, nil
}

// StreamEntry Redis Stream 条目
type StreamEntry struct {
    ID    string // Redis Stream 消息 ID（timestamp-seq）
    Event Event
}

// Trim 手动裁剪 Stream（可选，XADD 已配置 MAXLEN 自动裁剪）
func Trim(ctx context.Context, client *redis.Client, runID string, maxLen int64) error {
    return client.XTrimApprox(ctx, StreamKey(runID), maxLen).Err()
}

// Delete 清除 Run 的整个事件流（Run 结束后可选清理）
func Delete(ctx context.Context, client *redis.Client, runID string) error {
    return client.Del(ctx, StreamKey(runID)).Err()
}
```

### 5.2 Run Worker（Publisher 端）

Run Worker 负责执行 Agent 并将事件发布到 Redis Stream：

```go
package run

import (
    "context"
    "encoding/json"

    "Qavor/internal/eventbus"
)

// Worker Run 执行器（事件发布者）
type Worker struct {
    publisher *eventbus.Publisher
    // ... agentMgr, contextMgr, messageRepo 等依赖
}

// Execute 执行 Run 并发布事件
func (w *Worker) Execute(ctx context.Context, run *AgentRun) error {
    // 1. 发布 metadata 事件
    w.publish(ctx, run, eventbus.Event{
        EventType: "metadata",
        RunID:     run.ID,
        ThreadID:  run.ConversationThreadID,
        RequestID: run.RequestID,
        Payload:   mustJSON(map[string]any{
            "run_type": run.RunType,
            "source":   "chat",
            "agent_slug": run.AgentSlug,
        }),
    })

    // 2. 执行 Agent（使用 eino ADK 的 agent.Run() 事件迭代器）
    iter := agent.Run(ctx, input)
    for {
        event, ok := iter.Next()
        if !ok {
            break
        }
        if event.Err != nil {
            // 发布 error 事件
            w.publish(ctx, run, eventbus.Event{
                EventType: "error",
                RunID:     run.ID,
                Payload: mustJSON(map[string]any{
                    "error_type": "AGENT_ERROR",
                    "message":    event.Err.Error(),
                    "retryable":  false,
                }),
            })
            return event.Err
        }
        // 将 ADK 事件转换为 message 事件并发布
        w.publishChunk(ctx, run, event)
    }

    // 3. 发布 end 事件
    w.publish(ctx, run, eventbus.Event{
        EventType: "end",
        RunID:     run.ID,
        Payload: mustJSON(map[string]any{
            "status": "completed",
        }),
    })
    return nil
}

func (w *Worker) publish(ctx context.Context, run *AgentRun, event eventbus.Event) {
    msgID, err := w.publisher.Publish(ctx, run.ID, event)
    if err != nil {
        // 日志记录，不中断执行
    }
    // 更新 AgentRun.LastEventID
    run.LastEventID = msgID
}
```

### 5.3 POST Stream Handler（Subscriber 端）

POST Stream Handler 处理 `POST /api/agent/runs`，承担「新建 Run + 流式推送」与「resume 重连续传」两种语义，并起独立 goroutine 发送心跳注释行。

```go
package sse

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"

    "Qavor/internal/eventbus"
    "Qavor/internal/model/entity"
    "Qavor/internal/queue"
    "Qavor/internal/repository"

    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

// Handler POST 流式事件处理器（订阅者）
type Handler struct {
    subscriber       *eventbus.Subscriber
    runRepo          repository.AgentRunRepository
    reqQueue         queue.RequestQueue
    heartbeatPeriod  time.Duration // 心跳注释行周期
    logger           *zap.Logger
}

func NewHandler(sub *eventbus.Subscriber, runRepo repository.AgentRunRepository,
    reqQueue queue.RequestQueue, logger *zap.Logger) *Handler {
    return &Handler{
        subscriber:      sub,
        runRepo:         runRepo,
        reqQueue:        reqQueue,
        heartbeatPeriod: 15 * time.Second,
        logger:          logger,
    }
}

// CreateRunAndStream POST /api/agent/runs
// 新建 Run（resume 为空）或断线重连（resume 非空），均返回 SSE 流
func (h *Handler) CreateRunAndStream(c *gin.Context) {
    var req CreateRunRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
        return
    }

    var (
        run      *entity.AgentRun
        afterSeq string
        err      error
    )

    if req.Resume != nil {
        // —— 断线重连：校验已有 Run，不创建新 Run ——
        run, err = h.runRepo.GetByID(req.Resume.RunID)
        if err != nil || run.ConversationThreadID != req.ThreadID {
            c.JSON(http.StatusNotFound, gin.H{"code": 4004, "message": "run not found or expired"})
            return
        }
        afterSeq = req.Resume.LastSeq
    } else {
        // —— 新建 Run：创建记录并入队 ——
        run, err = h.createAndEnqueue(c, &req)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
            return
        }
        afterSeq = "0-0"
    }

    // 写 SSE 响应头并 Flush，立即发一次心跳让前端确认连接
    h.setSSEHeaders(c)
    h.writeHeartbeat(c)

    // 启动订阅循环（含心跳 goroutine）
    ctx := c.Request.Context()
    h.subscribeLoop(ctx, c, run, afterSeq)
}

func (h *Handler) setSSEHeaders(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")
    c.Writer.Flush()
}

func (h *Handler) writeHeartbeat(c *gin.Context) {
    // SSE 注释行：以 : 开头，前端自动忽略，不占用事件类型
    fmt.Fprint(c.Writer, ": heartbeat\n\n")
    if f, ok := c.Writer.(http.Flusher); ok {
        f.Flush()
    }
}

// subscribeLoop 事件订阅循环
// 主 goroutine 阻塞 XREAD；心跳 goroutine 周期写注释行保活
func (h *Handler) subscribeLoop(ctx context.Context, c *gin.Context, run *entity.AgentRun, afterSeq string) {
    flusher, _ := c.Writer.(http.Flusher)

    // 写锁：心跳 goroutine 与事件写入互斥，避免帧交错
    var writeMu sync.Mutex
    heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
    defer cancelHeartbeat()

    // 心跳 goroutine
    go func() {
        ticker := time.NewTicker(h.heartbeatPeriod)
        defer ticker.Stop()
        for {
            select {
            case <-heartbeatCtx.Done():
                return
            case <-ticker.C:
                writeMu.Lock()
                fmt.Fprint(c.Writer, ": heartbeat\n\n")
                if flusher != nil {
                    flusher.Flush()
                }
                writeMu.Unlock()
            }
        }
    }()

    var lastSeq = afterSeq
    for {
        select {
        case <-ctx.Done():
            return // 客户端断开
        default:
        }

        // Run 已终态且事件已全部推送
        if run.IsTerminal() && lastSeq >= run.LastEventID {
            return
        }

        entries, err := h.subscriber.Read(ctx, run.ID, lastSeq)
        if err != nil {
            h.logger.Error("读取事件失败", zap.Error(err))
            return
        }

        writeMu.Lock()
        for _, entry := range entries {
            terminal := h.writeSSEEvent(c, flusher, entry)
            lastSeq = entry.ID
            if terminal {
                writeMu.Unlock()
                return // 收到终态事件，关闭连接
            }
        }
        writeMu.Unlock()
    }
}

// writeSSEEvent 写入一个 SSE 事件，返回 true 表示终态事件
func (h *Handler) writeSSEEvent(c *gin.Context, flusher http.Flusher, entry eventbus.StreamEntry) bool {
    event := entry.Event
    envelope := map[string]any{
        "event":      event.EventType,
        "run_id":     event.RunID,
        "thread_id":  event.ThreadID,
        "request_id": event.RequestID,
        "payload":    json.RawMessage(event.Payload),
    }
    data, _ := json.Marshal(envelope)
    fmt.Fprintf(c.Writer, "id: %s\nevent: %s\ndata: %s\n\n",
        entry.ID, event.EventType, string(data))
    if flusher != nil {
        flusher.Flush()
    }
    return event.EventType == "end" || event.EventType == "error"
}
```

> **关键点**：
> - 心跳与事件写入共用一把 `sync.Mutex`，防止心跳注释行与事件帧字节交错
> - `XREAD BLOCK` 阻塞在独立循环，心跳 goroutine 不受其阻塞影响
> - 客户端断开（`ctx.Done()`）时同时取消心跳 goroutine，避免泄漏
> - 新建 Run 时先入队再开流；Run Worker 异步执行，事件经 Redis Stream 到达 POST Handler

### 5.4 Run API Controller

```go
package agent

import (
    "Qavor/internal/eventbus"
    "Qavor/internal/middleware"
    "Qavor/internal/queue"
    "Qavor/internal/repository"

    "github.com/gin-gonic/gin"
)

// RunController Run 相关接口控制器
type RunController struct {
    runRepo    repository.AgentRunRepository
    reqQueue   queue.RequestQueue
    sseHandler *sse.Handler
}

// RegisterRunRoutes 注册 Run 相关路由
func RegisterRunRoutes(r *gin.RouterGroup, ctrl *RunController) {
    runGroup := r.Group("/agent")
    runGroup.Use(middleware.Auth())
    {
        // 创建 Run 并直接返回 SSE 流（携带 resume 时为断线重连）
        runGroup.POST("/runs", ctrl.sseHandler.CreateRunAndStream)
        // 获取 Run 状态
        runGroup.GET("/runs/:runId", ctrl.GetRun)
        // 取消 Run
        runGroup.POST("/runs/:runId/cancel", ctrl.CancelRun)

        // 请求队列操作
        runGroup.GET("/requests/:requestId", ctrl.GetRequest)
        runGroup.POST("/requests/:requestId/cancel", ctrl.CancelRequest)
        runGroup.POST("/requests/:requestId/steer", ctrl.SteerRequest)
        runGroup.GET("/thread/:threadId/requests", ctrl.ListThreadRequests)
        runGroup.POST("/thread/:threadId/requests/continue", ctrl.ContinueThreadQueue)
    }
}
```

### 5.5 模块文件结构

```
internal/
├── eventbus/
│   ├── publisher.go         # 事件发布者（XADD）
│   ├── subscriber.go        # 事件订阅者（XREAD BLOCK）
│   └── event.go             # Event 结构定义
├── run/
│   ├── worker.go            # Run 执行器（调用 Agent，发布事件）
│   ├── state.go             # Run 状态机
│   └── queue.go             # 请求队列（enqueue/steer/cancel）
├── api/v1/agent/
│   ├── controller.go        # Agent CRUD
│   ├── run_controller.go    # Run 接口控制器（状态查询/取消/队列）
│   ├── post_stream_handler.go # POST 流式处理器（创建/重连 + SSE 推流 + 心跳）
│   └── routes.go            # 路由注册
├── sse/                     # 旧模块（保留兼容，逐步废弃）
│   ├── manager.go
│   ├── writer.go
│   └── ...
└── ...
```

---

## 六、前端实现

### 6.1 核心调用流程

前端通过 `useAgentRunStream` composable（`frontend/src/composables/useAgentRunStream.js`）封装，**单次 POST 直接获取 SSE 流**，核心流程：

```javascript
// 1. POST 创建 Run 并直接获取 SSE 流（fetch 流式读取）
const response = await agentApi.createAgentRunStream({
  query: userMessage,
  agent_slug: agentSlug,
  thread_id: threadId,
  queue_policy: 'enqueue'
}, { signal: abortController.signal })

// 2. 解析 SSE 流并处理事件（心跳注释行 : heartbeat 自动被解析器忽略）
await processRunSseResponse(response, (event, data, eventId) => {
  // eventId 为 Redis Stream seq，用于断线续传
  // event 为 metadata / message / end / error
  // data 为事件信封（含 payload.chunk / payload.items）
  switch (event) {
    case 'metadata':
      // Run 开始，从 data.run_id 拿到 runId 并存入快照
      threadState.activeRunId = data.run_id
      break
    case 'message':
      if (data.payload.items) {
        data.payload.items.forEach(handleChunk)
      } else if (data.payload.chunk) {
        handleChunk(data.payload.chunk)
      }
      break
    case 'end':
      if (data.payload.status === 'interrupted') {
        // 工具审批中断，保存快照等待恢复
      }
      break
    case 'error':
      break
  }
  // 每收到一个事件，更新 last_seq 快照
  threadState.runLastSeq = eventId
})

// 3. 断线重连：再次 POST，携带 resume（同一端点，不创建新 Run）
const lastSeq = threadState.runLastSeq // 如 "1234567-0"
const reconnectResp = await agentApi.createAgentRunStream({
  thread_id: threadId,
  resume: { run_id: threadState.activeRunId, last_seq: lastSeq }
})
await processRunSseResponse(reconnectResp, eventHandler)
```

> **fetch 而非 EventSource**：POST 请求无法用浏览器原生 `EventSource`（仅支持 GET），前端用 `fetch` + `ReadableStream` 解析 SSE 帧，注释行（`: heartbeat`）在解析时按 SSE 规范自动跳过。

### 6.2 断线续传机制

- **快照存储**：`localStorage` 保存 `active_run:{threadId}` 快照（含 `run_id`、`last_seq`、`created_at`）
- **重连逻辑**：`scheduleRunReconnect` 在 SSE 流异常关闭时延迟重连
- **续传位置**：重连 POST 携带 `resume: { run_id, last_seq }`，后端从 `last_seq` 之后续传
- **去重**：`compareRunSeq` 比较事件 seq，跳过已处理事件
- **TTL**：快照 1 小时过期，避免恢复过旧的 Run

### 6.3 前端 API 定义

前端 API 定义于 `frontend/src/apis/agent_api.js`：

| 方法 | 说明 |
|------|------|
| `agentApi.createAgentRunStream(data, options)` | POST 创建 Run / 断线重连，返回 SSE 流（fetch ReadableStream） |
| `agentApi.getAgentRun(runId)` | 获取 Run 状态 |
| `agentApi.cancelAgentRun(runId)` | 取消 Run |
| `agentApi.getRequest(requestId)` | 获取请求详情 |
| `agentApi.cancelRequest(requestId)` | 取消排队请求 |
| `agentApi.steerRequest(requestId)` | 引导请求 |
| `agentApi.listThreadQueuedRequests(threadId, agentSlug)` | 列出排队请求 |
| `agentApi.continueThreadQueue(threadId, agentSlug)` | 继续暂停队列 |

---

## 七、数据模型

### 7.1 AgentRun 实体（已存在）

```go
// internal/model/entity/agent_run.go
type AgentRun struct {
    ID                   string     `gorm:"primarykey"`           // Run ID (UUID)
    ConversationThreadID string     `gorm:"index"`                // 对话线程 ID
    AgentSlug            string     `gorm:"index"`                // Agent slug
    Status               string     `gorm:"default:pending"`      // 状态机
    RequestID            string     `gorm:"uniqueIndex"`          // 幂等性请求 ID
    RunType              string     `gorm:"default:chat"`         // chat / resume / subagent
    LastEventID          string     `gorm:"comment:Redis Stream最后事件ID"` // 续传位置
    InputPayload         JSON       `gorm:"type:json"`             // 原始输入
    ErrorType            string                                     // 错误类型
    ErrorMessage         string                                     // 错误信息
    StartedAt            *time.Time                                 // 开始时间
    FinishedAt           *time.Time                                 // 完成时间
    CreatedAt            time.Time
    UpdatedAt            time.Time
}
```

### 7.2 Redis Stream 结构

**Key**：`qavor:run:{runId}:events`

**消息格式**：
```
Stream: qavor:run:run-uuid-xxx:events
├── 1234567890123-0  Values: { "event": "{\"event\":\"metadata\",...}" }
├── 1234567890124-0  Values: { "event": "{\"event\":\"message\",...}" }
├── 1234567890125-0  Values: { "event": "{\"event\":\"end\",...}" }
MAXLEN ~ 10000  (近似裁剪，防止无限增长)
```

### 7.3 请求队列结构

**Key**：`qavor:thread:{threadId}:queue`（List 或 Sorted Set）

| 策略 | 行为 |
|------|------|
| `enqueue` | 入队排队，等待当前 Run 完成后执行 |
| `steer` | 提升为引导请求，下一条立即执行 |
| `cancel_previous` | 取消当前 Run，立即执行新请求 |

---

## 八、方案对比

### 8.1 最旧方案（单 POST 同步流式，无持久化）

```
POST /api/v1/chat/stream → 后端同步执行 Agent → 同一连接返回 SSE 流
```

| 问题 | 说明 |
|------|------|
| 同步阻塞 | HTTP 连接需保持到 Agent 执行结束，代理超时风险 |
| 无法恢复 | 连接断开后事件丢失，无法续传 |
| 无队列 | 不支持排队、引导、取消等操作 |
| 工具审批 | 不支持 interrupted 状态暂停 |
| 空闲断连 | 等待事件期间无数据，易被代理关闭 |

### 8.2 两步方案（POST 创建 Run + GET 订阅事件）

```
POST /api/agent/runs → 异步创建 Run，返回 runId
GET /api/agent/runs/:runId/events → 订阅 Redis Stream 事件
```

| 特点 | 说明 |
|------|------|
| 异步解耦 | Run 执行与 SSE 推送完全解耦 |
| 断线续传 | Redis Stream 持久化，通过 Last-Event-ID 恢复 |
| 多端同步 | 多个 SSE 连接可订阅同一 Run |
| 代价 | 前端需管理「先 POST 再 GET」两次请求，编排复杂 |

### 8.3 本方案（POST 单连接流式 + resume 续传 + 心跳）

```
POST /api/agent/runs → 创建 Run（或 resume 重连）并直接返回 SSE 流
```

| 优势 | 说明 |
|------|------|
| 单连接简化 | POST 创建 Run 后直接流式返回，前端一次请求搞定 |
| 异步解耦 | Run Worker 与 POST Handler 经 Redis Stream 解耦 |
| 断线续传 | `resume: { run_id, last_seq }` 重连，Redis Stream 持久化事件不丢失 |
| 队列支持 | 保留 enqueue/steer/cancel 等队列操作 |
| 工具审批 | 保留 interrupted 状态暂停与恢复 |
| 心跳保活 | SSE 注释行 `: heartbeat` 防止代理空闲断连 |
| 长任务 | 不受 HTTP 超时限制 |

> **取舍**：相比两步方案，本方案不依赖独立 GET 端点，无法原生支持「同一 Run 多端同时订阅」（单管理员体系下不需要）；多端同步如有需要，可后续在 POST 之外补一个 GET 重连端点，复用同一 Redis Stream。

---

## 九、测试策略

### 9.1 无用户体系说明

> **当前项目无多用户体系**，认证使用单一管理员账户（`configs/config.yaml` 中 `auth.admin_username` / `auth.admin_password`）。
> 测试时需要先通过登录接口获取 JWT Token，或在 dev 模式下使用测试 Token。

### 9.2 测试准备

#### 9.2.1 获取测试 Token

```bash
# 登录获取 JWT Token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "change-me", "password": "change-me"}'

# 响应
# { "code": 0, "data": { "token": "eyJhbG...", "expires_at": "..." } }
```

#### 9.2.2 环境要求

- Redis 已启动（`configs/config.yaml` 中 `database.redis.host`）
- PostgreSQL 已启动
- 至少一个 Agent 已配置（`agent_slug: "default"`）

### 9.3 单元测试

| 测试项 | 说明 |
|--------|------|
| Publisher 测试 | 验证 `XADD` 写入事件，返回正确的 Stream 消息 ID |
| Subscriber 测试 | 验证 `XREAD BLOCK` 读取事件，支持续传位置 |
| 续传测试 | 从中间 seq 读取，验证不重复、不遗漏 |
| 事件序列化 | 验证 Event JSON 序列化/反序列化正确 |
| Stream 裁剪 | 验证 MAXLEN 近似裁剪生效 |

### 9.4 集成测试

#### 9.4.1 创建 Run 并获取 SSE 流

```bash
TOKEN="eyJhbG..."

# POST 创建 Run 并直接获取 SSE 流（-N 禁用缓冲，实时输出）
curl -N -X POST http://localhost:8080/api/agent/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "你好",
    "agent_slug": "default",
    "thread_id": "test-thread-001",
    "queue_policy": "enqueue"
  }'
# 输出：: heartbeat 注释行 + id/event/data SSE 帧（metadata → message → end）
# run_id 从首个 metadata 事件的 data 中提取
```

#### 9.4.2 断线续传测试

```bash
# 第一次 POST 流式，记录最后的 id: 字段值
RUN_ID="run-xxx"      # 从 metadata 事件提取
LAST_ID="1234568-0"   # 从最后收到的 id: 提取

# 中断后，携带 resume 重连（同一 POST 端点，不创建新 Run）
curl -N -X POST http://localhost:8080/api/agent/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"thread_id\": \"test-thread-001\",
    \"resume\": { \"run_id\": \"$RUN_ID\", \"last_seq\": \"$LAST_ID\" }
  }"
# 从 last_seq 之后续传剩余事件
```

#### 9.4.3 取消 Run 测试

```bash
curl -X POST http://localhost:8080/api/agent/runs/$RUN_ID/cancel \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

#### 9.4.4 请求队列测试

```bash
# 创建多个 Run（排队）—— 注意 POST 直接返回 SSE 流，可用 -o /dev/null 丢弃流仅入队
curl -X POST http://localhost:8080/api/agent/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "问题1", "agent_slug": "default", "thread_id": "test-thread-001"}' \
  -o /dev/null

curl -X POST http://localhost:8080/api/agent/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "问题2", "agent_slug": "default", "thread_id": "test-thread-001"}' \
  -o /dev/null

# 查看排队请求
curl http://localhost:8080/api/agent/thread/test-thread-001/requests?agent_slug=default \
  -H "Authorization: Bearer $TOKEN"

# 引导第二个请求
REQUEST_ID="req-xxx"
curl -X POST http://localhost:8080/api/agent/requests/$REQUEST_ID/steer \
  -H "Authorization: Bearer $TOKEN" -d '{}'
```

### 9.5 端到端测试

| 测试场景 | 验证点 |
|----------|--------|
| 正常对话流 | POST 创建 Run → 直接收到 metadata + message + end |
| 打字机效果 | message 事件中 token chunk 逐个到达 |
| 工具调用 | message 事件中 tool_call_start + tool_call_result |
| 工具审批中断 | end 事件 status=interrupted，前端弹出审批 UI |
| 心跳保活 | 等待事件期间收到 `: heartbeat` 注释行，连接不断 |
| 断线续传 | 断开后 POST 携带 resume 恢复，事件不丢失 |
| 取消 Run | cancel 后收到 end 事件 status=cancelled |
| 队列排队 | 同一 thread 的多个 Run 依次执行 |
| 队列引导 | steer 后引导请求下一条执行 |

---

## 十、配置参数

### 10.1 Redis Stream 配置

```yaml
# configs/config.yaml 中新增
run:
  event_stream:
    max_len: 10000                    # 每个 Run 事件流最大长度（近似裁剪）
    block_timeout: 30s                # XREAD BLOCK 超时
    retention: 24h                    # Run 完成后事件流保留时长（过期清理）
  queue:
    max_per_thread: 10                # 每线程最大排队请求数
    stale_timeout: 5m                 # 排队请求超时
```

### 10.2 现有 SSE 配置（保留兼容）

```yaml
sse:
  max_stream_time: 300                # 单次流式最大时长（秒）
  heartbeat_interval: 15              # 心跳间隔（秒）
  max_concurrent_tasks: 5             # 单用户最大并发任务数
```

---

## 十一、注意事项

1. **Redis Stream 阻塞读**：POST Stream Handler 必须使用 `XREAD BLOCK`（非忙等待），避免 CPU 空转
2. **独立 goroutine**：每个 SSE 连接使用独立 goroutine 处理 `XREAD`，心跳用单独 goroutine + `sync.Mutex` 与事件写入互斥，避免帧字节交错
3. **代理缓冲**：Nginx 反向代理需禁用缓冲，并放大读超时以适配长任务 + 心跳：
   ```nginx
   proxy_buffering off;
   proxy_cache off;
   proxy_read_timeout 300s;
   ```
4. **心跳周期**：`sse.heartbeat_interval`（默认 15s）需小于代理空闲超时；心跳为 SSE 注释行 `: heartbeat`，前端解析时按规范自动忽略
5. **POST 不可用 EventSource**：浏览器原生 `EventSource` 仅支持 GET，前端必须用 `fetch` + `ReadableStream` 解析 SSE 帧；续传位置通过请求体 `resume.last_seq` 携带，不依赖 `Last-Event-ID` 请求头
6. **CORS**：POST 流式需允许 `Content-Type: text/event-stream` 响应跨域；若改回 GET 重连端点则需将 `Last-Event-ID` 加入 CORS 允许头
7. **Stream 清理**：Run 终态后延迟清理 Redis Stream（配置 `retention`，默认 24h），支持短时间内的 resume 续传恢复
8. **事件顺序**：Redis Stream 保证同 Run 内事件按 timestamp-seq 有序
9. **resume 校验**：重连时必须校验 `resume.run_id` 存在、归属当前 `thread_id`、未过 TTL，防止越权或恢复过旧 Run
10. **无用户体系**：当前所有 Run 归属 `admin`，后续引入用户体系时需在 Run 实体增加 `owner` 字段
