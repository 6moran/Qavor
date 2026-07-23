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
| 1003 | 用户名或密码错误 |
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
  "username": "testuser",
  "password": "123456",
  "email": "test@example.com",
  "nickname": "测试用户"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名（3-50字符） |
| password | string | 是 | 密码（6-50字符） |
| email | string | 是 | 邮箱地址 |
| nickname | string | 否 | 昵称（最多50字符） |

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
  "username": "testuser",
  "password": "123456"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名 |
| password | string | 是 | 密码 |

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "test@example.com",
      "nickname": "测试用户",
      "avatar": "",
      "status": 1,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  }
}
```

#### 刷新 Token

```http
POST /api/v1/auth/refresh
```

**请求参数**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

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

---

### 用户管理

以下接口需要认证，请在请求头中携带 Token。

#### 获取当前用户信息

```http
GET /api/v1/user/profile
Authorization: Bearer {token}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "id": 1,
    "username": "testuser",
    "email": "test@example.com",
    "nickname": "测试用户",
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
  "nickname": "新昵称",
  "avatar": "https://example.com/avatar.jpg"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| nickname | string | 否 | 昵称 |
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
        "username": "testuser",
        "email": "test@example.com",
        "nickname": "测试用户",
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
  -d '{"username":"test","password":"123456","email":"test@example.com"}'

# 用户登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"123456"}'

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
    username: 'test',
    password: '123456',
  }),
});

const data = await response.json();
const token = data.data.token;

// 获取用户信息
const profile = await fetch('http://localhost:8080/api/v1/user/profile', {
  headers: {
    'Authorization': `Bearer ${token}`,
  },
});

const profileData = await profile.json();

// 获取用户列表（分页）
const users = await fetch('http://localhost:8080/api/v1/user/list?page=1&size=10', {
  headers: {
    'Authorization': `Bearer ${token}`,
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
    'username': 'test',
    'password': '123456',
})

data = response.json()
token = data['data']['token']

# 获取用户信息
profile = requests.get('http://localhost:8080/api/v1/user/profile', headers={
    'Authorization': f'Bearer {token}',
})

profile_data = profile.json()

# 获取用户列表（分页）
users = requests.get('http://localhost:8080/api/v1/user/list', params={
    'page': 1,
    'size': 10
}, headers={
    'Authorization': f'Bearer {token}',
})

users_data = users.json()
print(users_data['data']['list'])  # 用户列表
print(users_data['data']['total'])  # 总记录数
print(users_data['data']['total_page'])  # 总页数
```

---

## 注意事项

1. **Token 有效期**: Token 默认有效期为 24 小时
2. **密码安全**: 密码使用 bcrypt 加密存储，服务端无法查看明文密码
3. **请求频率**: 建议客户端实现请求频率限制
4. **错误处理**: 请根据错误码进行相应的错误处理
5. **时区**: 所有时间使用 UTC 时区，格式为 ISO 8601
