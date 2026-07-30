#!/bin/bash
# 模型提供商 API 测试（无需认证）

BASE_URL="http://localhost:8080/api/v1"

echo "=== 测试模型提供商 API（无需认证）==="
echo ""

# 1. 健康检查
echo "1. 健康检查"
curl -s "$BASE_URL/health" | python -m json.tool 2>/dev/null || curl -s "$BASE_URL/health"
echo ""
echo ""

# 2. 获取供应商列表（应该返回空列表或测试数据）
echo "2. 获取供应商列表"
curl -s "$BASE_URL/model-providers" | python -m json.tool 2>/dev/null || curl -s "$BASE_URL/model-providers"
echo ""
echo ""

# 3. 创建供应商（无需认证）
echo "3. 创建 OpenAI 供应商"
curl -s -X POST "$BASE_URL/model-providers" \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": "openai",
    "display_name": "OpenAI",
    "provider_type": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key_env": "OPENAI_API_KEY",
    "capabilities": ["chat", "embedding"],
    "enabled_models": [
      {"model_id": "gpt-4o", "display_name": "GPT-4o", "type": "chat"}
    ]
  }' | python -m json.tool 2>/dev/null || \
curl -s -X POST "$BASE_URL/model-providers" \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": "openai",
    "display_name": "OpenAI",
    "provider_type": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key_env": "OPENAI_API_KEY",
    "capabilities": ["chat", "embedding"],
    "enabled_models": [
      {"model_id": "gpt-4o", "display_name": "GPT-4o", "type": "chat"}
    ]
  }'
echo ""
echo ""

# 4. 再次获取列表
echo "4. 再次获取供应商列表"
curl -s "$BASE_URL/model-providers" | python -m json.tool 2>/dev/null || curl -s "$BASE_URL/model-providers"
echo ""
echo ""

echo "=== 测试完成 ==="
echo ""
echo "如果看到 404 错误，请检查："
echo "1. 服务是否已启动 (go run main.go)"
echo "2. 端口是否正确 (默认 8080)"
echo "3. 路径是否正确 (/api/v1/model-providers)"
