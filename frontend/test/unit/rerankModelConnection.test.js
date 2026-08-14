import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildModelTestPayload,
  isModelConnectionTestSupported
} from '../../src/utils/modelConfig.js'

test('allows rerank connection testing and keeps the model type in payload', () => {
  assert.equal(isModelConnectionTestSupported('rerank'), true)

  const payload = buildModelTestPayload({
    name: 'bge-reranker-v2-m3',
    remark: '',
    protocol: 'openai',
    base_url: 'https://rerank.example.com',
    api_key: 'secret',
    headers: '{}',
    timeout: 60000,
    enabled: true,
    model_type: 'rerank',
    params: '{}'
  })

  assert.equal(payload.model_type, 'rerank')
})
