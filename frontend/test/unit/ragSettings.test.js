import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildRagSettingsPayload,
  normalizeRagSettingsResponse,
  persistRerankSelection
} from '../../src/utils/rag_settings.js'

test('normalizes disabled and configured RAG settings', () => {
  assert.deepEqual(normalizeRagSettingsResponse({ data: { rerank_model_id: null } }), {
    rerankModelId: null,
    rerankModelName: ''
  })
  assert.deepEqual(
    normalizeRagSettingsResponse({
      data: { rerank_model_id: 7, rerank_model_name: 'bge-reranker-v2-m3' }
    }),
    { rerankModelId: 7, rerankModelName: 'bge-reranker-v2-m3' }
  )
})

test('builds the dedicated RAG settings payload', () => {
  assert.deepEqual(buildRagSettingsPayload(7), { rerank_model_id: 7 })
  assert.deepEqual(buildRagSettingsPayload(null), { rerank_model_id: null })
  assert.deepEqual(buildRagSettingsPayload(undefined), { rerank_model_id: null })
})

test('restores the previous selection when persistence fails', async () => {
  const previous = { rerankModelId: 7, rerankModelName: '旧模型' }
  const failure = new Error('保存失败')
  const result = await persistRerankSelection({
    previous,
    nextModelId: 8,
    update: async () => {
      throw failure
    }
  })

  assert.deepEqual(result.settings, previous)
  assert.equal(result.error, failure)
})
