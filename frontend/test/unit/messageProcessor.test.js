import assert from 'node:assert/strict'
import test from 'node:test'

import { MessageProcessor } from '../../src/utils/messageProcessor.js'

test('交付物只归属于调用 present_artifacts 的对话', () => {
  const artifactConversation = {
    messages: [
      {
        type: 'ai',
        tool_calls: [
          {
            name: 'present_artifacts',
            tool_call_result: { content: '已将交付物展示给用户' },
            args: JSON.stringify({
              filepaths: [
                '/home/gem/user-data/outputs/bubble_sort.py',
                '/home/gem/user-data/outputs/bubble_sort.js'
              ]
            })
          },
          {
            function: { name: 'present_artifacts' },
            status: 'success',
            args: { filepaths: ['/home/gem/user-data/outputs/bubble_sort.py'] }
          }
        ]
      }
    ]
  }
  const laterConversation = {
    messages: [{ type: 'human', content: '运行 Python 的' }]
  }

  assert.deepEqual(MessageProcessor.extractArtifactsFromConversation(artifactConversation), [
    '/home/gem/user-data/outputs/bubble_sort.py',
    '/home/gem/user-data/outputs/bubble_sort.js'
  ])
  assert.deepEqual(MessageProcessor.extractArtifactsFromConversation(laterConversation), [])
})

test('query_kb 顶层 chunks 会转换为知识来源并按 kb_id 回退名称', () => {
  const conversation = {
    messages: [
      {
        type: 'ai',
        tool_calls: [
          {
            name: 'query_kb',
            tool_call_result: {
              content: JSON.stringify({
                query_text: '测试问题',
                chunks: [
                  {
                    kb_id: 'kb-1',
                    kb_name: '结果内知识库',
                    file_id: 'file-1',
                    chunk_id: 'chunk-1',
                    filename: '产品说明.md',
                    content: '第一段命中内容',
                    score: 0.91
                  },
                  {
                    kb_id: 'kb-2',
                    file_id: 'file-2',
                    chunk_id: 'chunk-2',
                    filename: '使用手册.md',
                    content: '第二段命中内容',
                    score: 0.82
                  }
                ]
              })
            }
          }
        ]
      },
      { type: 'ai', content: '最终回答', isLast: true }
    ]
  }
  const databases = [
    { kb_id: 'kb-1', name: '列表中的旧名称' },
    { kb_id: 'kb-2', name: '回退知识库' }
  ]

  const sources = MessageProcessor.extractSourcesFromConversation(conversation, databases)

  assert.equal(sources.knowledgeChunks.length, 2)
  assert.equal(sources.knowledgeChunks[0].kb_name, '结果内知识库')
  assert.equal(sources.knowledgeChunks[0].metadata.source, '产品说明.md')
  assert.equal(sources.knowledgeChunks[0].metadata.chunk_id, 'chunk-1')
  assert.equal(sources.knowledgeChunks[1].kb_name, '回退知识库')
  assert.equal(sources.knowledgeChunks[1].metadata.file_id, 'file-2')
})

test('query_kb 来源按整轮对话去重而不是只读取最终回答', () => {
  const result = JSON.stringify({
    chunks: [
      {
        kb_id: 'kb-1',
        kb_name: '项目知识库',
        file_id: 'file-1',
        chunk_id: 'chunk-1',
        filename: 'README.md',
        content: '同一个知识片段',
        score: 0.9
      }
    ]
  })
  const conversation = {
    messages: [
      {
        type: 'ai',
        tool_calls: [{ name: 'query_kb', tool_call_result: { content: result } }]
      },
      {
        type: 'ai',
        tool_calls: [{ name: 'query_kb', tool_call_result: { content: result } }]
      },
      { type: 'ai', content: '最终回答', isLast: true }
    ]
  }

  const sources = MessageProcessor.extractSourcesFromConversation(conversation, [
    { kb_id: 'kb-1', name: '项目知识库' }
  ])

  assert.equal(sources.knowledgeChunks.length, 1)
  assert.equal(sources.knowledgeChunks[0].content, '同一个知识片段')
})
