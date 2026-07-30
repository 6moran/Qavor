#!/bin/bash
# 模型 API 测试示例（单一实体设计）

BASE_URL="http://localhost:8080/api/v1"

echo "=== 模型 API 测试示例 ==="
echo ""

# 1. 创建 OpenAI 模型
echo "1. 创建 OpenAI 模型"
curl -s -X POST "$BASE_URL/models" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "OpenAI",
    "protocol": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-your-openai-api-key",
    "timeout": 60000,
    "enabled": true,
    "model_type": "chat",
    "max_tokens": 128000
  }' | python -m json.tool
echo ""
echo ""

# 2. 创建 Anthropic 模型
echo "2. 创建 Anthropic (Claude) 模型"
curl -s -X POST "$BASE_URL/models" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Anthropic (Claude)",
    "protocol": "anthropic",
    "base_url": "https://api.anthropic.com",
    "api_key": "sk-ant-your-anthropic-api-key",
    "headers": {"anthropic-version": "2023-06-01"},
    "timeout": 60000,
    "enabled": true,
    "model_type": "chat",
    "max_tokens": 200000
  }' | python -m json.tool
echo ""
echo ""

# 3. 创建 Ollama 模型
echo "3. 创建 Ollama 本地模型"
curl -s -X POST "$BASE_URL/models" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Ollama (本地)",
    "protocol": "ollama",
    "base_url": "http://localhost:11434/v1",
    "api_key": "ollama",
    "timeout": 30000,
    "enabled": true,
    "model_type": "chat",
    "max_tokens": 128000
  }' | python -m json.tool
echo ""
echo ""

# 4. 获取模型列表
echo "4. 获取模型列表"
curl -s "$BASE_URL/models" | python -m json.tool
echo ""
echo ""

# 5. 获取单个模型
echo "5. 获取单个模型 (ID=1)"
curl -s "$BASE_URL/models/1" | python -m json.tool
echo ""
echo ""

# 6. 更新模型
echo "6. 更新模型 (ID=1)"
curl -s -X PUT "$BASE_URL/models/1" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "OpenAI (已更新)",
    "enabled": true
  }' | python -m json.tool
echo ""
echo ""

# 7. 按类型筛选模型
echo "7. 获取 chat 类型的模型"
curl -s "$BASE_URL/models?model_type=chat" | python -m json.tool
echo ""
echo ""

# 8. 搜索模型
echo "8. 搜索包含 'Claude' 的模型"
curl -s "$BASE_URL/models?keyword=Claude" | python -m json.tool
echo ""
echo ""

# 9. 删除模型
echo "9. 删除模型 (ID=2)"
curl -s -X DELETE "$BASE_URL/models/2" | python -m json.tool
echo ""
echo ""

echo "=== 测试完成 ==="
