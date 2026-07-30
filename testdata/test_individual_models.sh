#!/bin/bash
# 模型提供商 API 测试 - 逐个添加模型

BASE_URL="http://localhost:8080/api/v1"

echo "=== 测试逐个添加模型 API ==="
echo ""

# 1. 创建供应商（不带 enabled_models）
echo "1. 创建 OpenAI 供应商（不带模型）"
curl -s -X POST "$BASE_URL/model-providers" \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": "openai",
    "display_name": "OpenAI",
    "provider_type": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key_env": "OPENAI_API_KEY",
    "capabilities": ["chat", "embedding"]
  }' | python -m json.tool
echo ""
echo ""

# 2. 逐个添加模型
echo "2. 添加 GPT-4o 模型"
curl -s -X POST "$BASE_URL/model-providers/1/models" \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "gpt-4o",
    "display_name": "GPT-4o",
    "type": "chat",
    "max_tokens": 128000
  }' | python -m json.tool
echo ""
echo ""

echo "3. 添加 GPT-4o-mini 模型"
curl -s -X POST "$BASE_URL/model-providers/1/models" \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "gpt-4o-mini",
    "display_name": "GPT-4o Mini",
    "type": "chat",
    "max_tokens": 128000
  }' | python -m json.tool
echo ""
echo ""

echo "4. 添加 Embedding 模型"
curl -s -X POST "$BASE_URL/model-providers/1/models" \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "text-embedding-3-large",
    "display_name": "Text Embedding 3 Large",
    "type": "embedding",
    "dimensions": 3072
  }' | python -m json.tool
echo ""
echo ""

# 3. 查看供应商详情
echo "5. 查看供应商详情"
curl -s "$BASE_URL/model-providers/1" | python -m json.tool
echo ""
echo ""

# 4. 移除模型
echo "6. 移除 GPT-4o-mini 模型"
curl -s -X DELETE "$BASE_URL/model-providers/1/models" \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "gpt-4o-mini"
  }' | python -m json.tool
echo ""
echo ""

# 5. 再次查看
echo "7. 再次查看供应商详情"
curl -s "$BASE_URL/model-providers/1" | python -m json.tool
echo ""
echo ""

echo "=== 测试完成 ==="
