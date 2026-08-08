# API 文档

## 基础信息

- **Base URL**: `http://localhost:8080`
- **API 版本**: v1
- **Content-Type**: `application/json`

## 统一响应格式

所有 API 返回统一的响应格式：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

- `code`: 状态码，0 表示成功
- `message`: 响应消息
- `data`: 响应数据

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未授权 |
| 403 | 禁止访问 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |
| 1001 | 用户不存在 |
| 1002 | 用户已存在 |
| 1003 | 邮箱或密码错误 |
| 1004 | 用户已被禁用 |
| 1005 | 无效的令牌 |
| 1006 | 令牌已过期 |

## 认证方式

使用 JWT Bearer Token 认证：

```
Authorization: Bearer {token}
```

## API 接口

### 健康检查

#### 检查服务状态

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

### 用户认证

#### 用户注册

```http
POST /api/v1/auth/register
```

**请求参数**:
```json
{
  "nickname": "测试用户",
  "password": "123456",
  "confirm_password": "123456",
  "email": "test@example.com"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| nickname | string | 是 | 昵称（2-50字符，用于显示） |
| password | string | 是 | 密码（6-50字符） |
| confirm_password | string | 是 | 确认密码，必须与密码一致 |
| email | string | 是 | 邮箱地址（用于登录） |

**说明**: 系统自动生成 UID（格式：`usr_<UUID>`）

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

#### 用户登录

```http
POST /api/v1/auth/login
```

**请求参数**:
```json
{
  "email": "test@example.com",
  "password": "123456"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| email | string | 是 | 邮箱地址 |
| password | string | 是 | 密码 |

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "access_expires_in": 7200,
    "refresh_expires_in": 604800,
    "user": {
      "id": 1,
      "nickname": "测试用户",
      "uid": "usr_550e8400-e29b-41d4-a716-446655440000",
      "email": "test@example.com",
      "avatar": "",
      "status": 1,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  }
}
```

#### 用户登出

```http
POST /api/v1/auth/logout
Authorization: Bearer {access_token}
```

**请求参数**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

#### 刷新 Token

```http
POST /api/v1/auth/refresh
```

**请求参数**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "access_expires_in": 7200,
    "refresh_expires_in": 604800
  }
}
```

---

### 密码重置

#### 发送重置验证码

```http
POST /api/v1/auth/reset-code/send
```

**请求参数**:
```json
{
  "email": "user@example.com"
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "expires_in": 600
  }
}
```

#### 重置密码

```http
POST /api/v1/auth/password/reset
```

**请求参数**:
```json
{
  "email": "user@example.com",
  "code": "123456",
  "new_password": "newpassword123",
  "confirm_password": "newpassword123"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| email | string | 是 | 邮箱地址 |
| code | string | 是 | 6 位数字验证码 |
| new_password | string | 是 | 新密码（6-50字符） |
| confirm_password | string | 是 | 确认密码，必须与新密码一致 |

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

---

### 用户管理

以下接口需要认证，请在请求头中携带 Token。

#### 获取当前用户信息

```http
GET /api/v1/user/profile
Authorization: Bearer {access_token}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "id": 1,
    "nickname": "测试用户",
    "uid": "usr_550e8400-e29b-41d4-a716-446655440000",
    "email": "test@example.com",
    "avatar": "",
    "status": 1,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

#### 更新用户信息

```http
PUT /api/v1/user/profile
Authorization: Bearer {token}
```

**请求参数**:
```json
{
  "avatar": "https://example.com/avatar.jpg"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| avatar | string | 否 | 头像 URL |

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

#### 修改密码

```http
POST /api/v1/user/password
Authorization: Bearer {token}
```

**请求参数**:
```json
{
  "old_password": "123456",
  "new_password": "654321"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| old_password | string | 是 | 旧密码 |
| new_password | string | 是 | 新密码（6-50字符） |

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

#### 获取用户列表（分页）

```http
GET /api/v1/user/list?page=1&size=10
Authorization: Bearer {token}
```

**查询参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 是 | 页码，从 1 开始 |
| size | int | 是 | 每页大小，1-100 |

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "list": [
      {
        "id": 1,
        "nickname": "测试用户",
        "uid": "usr_550e8400-e29b-41d4-a716-446655440000",
        "email": "test@example.com",
        "avatar": "",
        "status": 1,
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
      }
    ],
    "total": 100,
    "page": 1,
    "size": 10,
    "total_page": 10
  }
}
```

**响应字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| list | array | 用户数据列表 |
| total | int64 | 总记录数 |
| page | int | 当前页码 |
| size | int | 每页大小 |
| total_page | int | 总页数 |

---

## 使用示例

### cURL

```bash
# 用户注册
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"nickname":"测试用户","password":"123456","email":"test@example.com"}'

# 用户登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"123456"}'

# 获取用户信息
curl http://localhost:8080/api/v1/user/profile \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"

# 获取用户列表（分页）
curl "http://localhost:8080/api/v1/user/list?page=1&size=10" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

### JavaScript (Fetch)

```javascript
// 用户登录
const response = await fetch('http://localhost:8080/api/v1/auth/login', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    email: 'test@example.com',
    password: '123456',
  }),
});

const data = await response.json();
const accessToken = data.data.access_token;
const refreshToken = data.data.refresh_token;

// 获取用户信息
const profile = await fetch('http://localhost:8080/api/v1/user/profile', {
  headers: {
    'Authorization': `Bearer ${accessToken}`,
  },
});

const profileData = await profile.json();

// 获取用户列表（分页）
const users = await fetch('http://localhost:8080/api/v1/user/list?page=1&size=10', {
  headers: {
    'Authorization': `Bearer ${accessToken}`,
  },
});

const usersData = await users.json();
console.log(usersData.data.list); // 用户列表
console.log(usersData.data.total); // 总记录数
console.log(usersData.data.total_page); // 总页数
```

### Python (requests)

```python
import requests

# 用户登录
response = requests.post('http://localhost:8080/api/v1/auth/login', json={
    'email': 'test@example.com',
    'password': '123456',
})

data = response.json()
access_token = data['data']['access_token']
refresh_token = data['data']['refresh_token']

# 获取用户信息
profile = requests.get('http://localhost:8080/api/v1/user/profile', headers={
    'Authorization': f'Bearer {access_token}',
})

profile_data = profile.json()

# 获取用户列表（分页）
users = requests.get('http://localhost:8080/api/v1/user/list', params={
    'page': 1,
    'size': 10
}, headers={
    'Authorization': f'Bearer {access_token}',
})

users_data = users.json()
print(users_data['data']['list'])  # 用户列表
print(users_data['data']['total'])  # 总记录数
print(users_data['data']['total_page'])  # 总页数
```

---

## 链路追踪（Trace）

Agent 对话执行链路追踪：通过 eino Callback 全局采集 LLM / Tool / Retriever / Agent 组件调用，生成 Trace 与 Span 记录，供开发者排查 Agent 执行链路。

### 获取 Trace 列表

`GET /api/v1/traces`（需认证）

**Query 参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| keyword | string | 按问题关键词模糊搜索 |
| agent_slug | string | 按 Agent 筛选 |
| conversation_id | int | 按会话 ID 筛选 |
| status | string | running / success / failed / cancelled / timeout |
| source | string | sync（同步）/ stream（流式）/ run（异步 Run） |
| from | string | 开始时间（RFC3339） |
| to | string | 结束时间（RFC3339） |
| page | int | 页码，默认 1 |
| page_size | int | 每页条数，默认 20，最大 100 |

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "trace_id": "3f0c...",
        "source": "stream",
        "agent_slug": "assistant",
        "query": "什么是退款政策？",
        "status": "success",
        "duration_ms": 2340,
        "model_name": "gpt-4o",
        "total_tokens": 356,
        "started_at": "2026-08-08T10:00:00Z",
        "ended_at": "2026-08-08T10:00:02Z"
      }
    ],
    "total": 1
  }
}
```

### 获取 Trace 详情

`GET /api/v1/traces/:trace_id`（需认证）

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
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
- 数据保留天数与超时标记由 `config.yaml` 的 `trace` 段配置，janitor 定期清理

---

## 注意事项

1. **双 Token 机制**: 
   - `access_token`: 访问令牌，有效期 2 小时
   - `refresh_token`: 刷新令牌，有效期 7 天
2. **用户标识**: 系统自动生成 UID（格式：`usr_<UUID>`），用于内部标识
3. **密码安全**: 密码使用 bcrypt 加密存储，服务端无法查看明文密码
4. **请求频率**: 建议客户端实现请求频率限制
5. **错误处理**: 请根据错误码进行相应的错误处理
6. **时区**: 所有时间使用 UTC 时区，格式为 ISO 8601
