# 模型测试数据

本目录包含用于测试模型 API 的测试数据。

## 文件说明

### 1. `models.json`
包含多个常见模型的测试数据：

- **OpenAI** - GPT 系列模型
- **Anthropic (Claude)** - Claude 系列模型
- **Ollama (本地)** - 本地部署的开源模型
- **DeepSeek** - DeepSeek 系列模型
- **智谱 AI (GLM)** - GLM 系列模型
- **SiliconFlow (硅基流动)** - 多种开源模型

### 2. `api_test_examples.sh`
API 测试脚本，包含创建、查询、更新、删除模型的 curl 命令示例。

## 数据结构

### 模型 (`Model`)

```json
{
  "id": 1,
  "name": "OpenAI",
  "protocol": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-xxx",
  "org_id": "",
  "headers": {},
  "timeout": 60000,
  "enabled": true,
  "model_type": "chat",
  "max_tokens": 128000,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint | 主键 ID |
| `name` | string | 服务商展示名（如 OpenAI / Anthropic / MiMo） |
| `protocol` | string | 协议类型：openai / anthropic / ollama / custom |
| `base_url` | string | API 地址 |
| `api_key` | string | 密钥（加密存储） |
| `org_id` | string | 组织 ID（OpenAI 专属） |
| `headers` | map | 自定义请求头 |
| `timeout` | int | 超时时间（毫秒） |
| `enabled` | bool | 是否启用 |
| `model_type` | string | 模型类型：chat / embedding / rerank |
| `max_tokens` | int | 最大 token 数 |

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/models` | 创建模型 |
| GET | `/api/v1/models` | 获取模型列表 |
| GET | `/api/v1/models/:id` | 获取模型详情 |
| PUT | `/api/v1/models/:id` | 更新模型 |
| DELETE | `/api/v1/models/:id` | 删除模型 |

## 使用示例

### 1. 创建模型

```bash
curl -X POST http://localhost:8080/api/v1/models \
  -H "Content-Type: application/json" \
  -d '{
    "name": "OpenAI",
    "protocol": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-your-api-key",
    "timeout": 60000,
    "enabled": true,
    "model_type": "chat",
    "max_tokens": 128000
  }'
```

### 2. 获取模型列表

```bash
# 获取所有模型
curl http://localhost:8080/api/v1/models

# 获取特定类型的模型
curl "http://localhost:8080/api/v1/models?model_type=chat"

# 搜索模型
curl "http://localhost:8080/api/v1/models?keyword=OpenAI"
```

### 3. 更新模型

```bash
curl -X PUT http://localhost:8080/api/v1/models/1 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "OpenAI (已更新)",
    "enabled": true
  }'
```

### 4. 删除模型

```bash
curl -X DELETE http://localhost:8080/api/v1/models/1
```

## 设计特点

1. **单一实体**：每个模型就是一个完整的接入点，包含所有连接信息
2. **API Key 加密存储**：使用 AES 加密存储敏感信息
3. **前端友好**：查看和添加使用同一个表单结构
4. **支持多种协议**：openai / anthropic / ollama / custom
5. **灵活配置**：每个模型可以有独立的 base-url、api-key、headers 等

## 注意事项

1. API Key 会被加密存储，前端查看时会显示为 `****`
2. `timeout` 单位为毫秒，默认 60000（60秒）
3. `model_type` 可选值：`chat`、`embedding`、`rerank`
4. `max_tokens` 默认 4096，可根据模型类型调整
