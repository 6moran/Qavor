import assert from 'node:assert/strict'
import test from 'node:test'
import { buildKnowledgeBaseCreateRequest } from '../../src/utils/knowledge_base_create.js'

test('builds a fixed knowledge base request without a type field', () => {
  const request = buildKnowledgeBaseCreateRequest({
    name: '  产品文档  ',
    description: '  产品说明  ',
    embedding_model_id: 1,
    chat_model_id: 2,
    rerank_model_id: 7,
    chunk_preset_id: 'default'
  })

  assert.deepEqual(request, {
    database_name: '产品文档',
    description: '产品说明',
    embedding_model_id: 1,
    chat_model_id: 2,
    additional_params: { chunk_preset_id: 'default' }
  })
  assert.equal(Object.hasOwn(request, 'kb_type'), false)
  assert.equal(Object.hasOwn(request, 'rerank_model_id'), false)
})
