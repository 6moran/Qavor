import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

// 构建索引请求的辅助函数（纯函数，无 Vue 依赖）
import { buildIndexDocumentsRequest } from '../../src/utils/build_index_request.js'

/**
 * 以 FileUploadModal 相同的方式构建上传查询字符串。
 * 这是上传参数逻辑的纯函数镜像，可以在不挂载 Vue 组件的情况下进行测试。
 */
const buildKnowledgeUploadQuery = ({ kbId, parentId } = {}) => {
  const params = new URLSearchParams()
  if (kbId) params.set('kb_id', kbId)
  if (parentId) params.set('parent_id', parentId)
  return Object.fromEntries(params.entries())
}

describe('buildIndexDocumentsRequest', () => {
  it('encodes kbId in URL and whitelists only chunk params', () => {
    const request = buildIndexDocumentsRequest('kb 1', ['file-1'], {
      chunk_preset_id: 'general',
      chunk_parser_config: { chunk_token_num: 500, overlapped_percent: 10 },
      embedding_model_id: 99
    })
    assert.equal(request.url, '/api/v1/knowledge/databases/kb%201/documents/index')
    assert.deepEqual(request.body.file_ids, ['file-1'])
    assert.equal('embedding_model_id' in request.body.params, false)
    assert.equal(request.body.params.chunk_preset_id, 'general')
  })

  it('is used by the production document API', () => {
    const source = readFileSync(new URL('../../src/apis/knowledge_api.js', import.meta.url), 'utf8')
    assert.match(source, /const \{ url, body \} = buildIndexDocumentsRequest\(kbId, fileIds, params\)/)
  })
})

describe('buildKnowledgeUploadQuery', () => {
  it('only includes kb_id and parent_id, never auto_index', () => {
    const query = buildKnowledgeUploadQuery({ kbId: 'kb-1', parentId: 'folder-1' })
    assert.equal(query.kb_id, 'kb-1')
    assert.equal(query.parent_id, 'folder-1')
    assert.equal('auto_index' in query, false)
  })

  it('omits parent_id when not provided', () => {
    const query = buildKnowledgeUploadQuery({ kbId: 'kb-1' })
    assert.equal(query.kb_id, 'kb-1')
    assert.equal('parent_id' in query, false)
    assert.equal('auto_index' in query, false)
  })
})
