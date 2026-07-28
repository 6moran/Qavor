import assert from 'node:assert/strict'

import { MessageProcessor } from '../messageProcessor.js'

const databases = [{ name: '财税库' }, { name: 'DifyKB' }, { name: 'LightGraphKB' }]

const run = () => {
  const conv = {
    messages: [
      {
        type: 'ai',
        tool_calls: [
          {
            name: '财税库',
            tool_call_result: {
              content: JSON.stringify([
                {
                  content: 'A',
                  score: 0.9,
                  metadata: { source: 'doc-a', chunk_id: 'c1', file_id: 'f1', chunk_index: 1 }
                },
                {
                  content: 'A',
                  score: 0.8,
                  metadata: { source: 'doc-a', chunk_id: 'c1', file_id: 'f1', chunk_index: 1 }
                }
              ])
            }
          },
          {
            name: 'LightGraphKB',
            tool_call_result: {
              content: JSON.stringify({
                data: {
                  chunks: [
                    {
                      content: 'B',
                      score: 0.4,
                      metadata: { source: 'doc-b', chunk_id: 'c2', file_id: 'f2', chunk_index: 2 }
                    }
                  ]
                }
              })
            }
          },
          {
            name: 'not_kb_tool',
            tool_call_result: {
              content: JSON.stringify([{ content: 'X', score: 0.99, metadata: { chunk_id: 'cx' } }])
            }
          },
          {
            name: 'DifyKB',
            tool_call_result: { content: 'not-json' }
          }
        ]
      }
    ]
  }

  const chunks = MessageProcessor.extractKnowledgeChunksFromConversation(conv, databases)

  // 1. Milvus/Dify 数组提取
  assert.equal(
    chunks.some((c) => c.content === 'A' && c.kb_name === '财税库'),
    true
  )

  // 2. 对象包装的 data.chunks 提取
  assert.equal(
    chunks.some((c) => c.content === 'B' && c.kb_name === 'LightGraphKB'),
    true
  )

  // 3. 非知识库工具忽略
  assert.equal(
    chunks.some((c) => c.content === 'X'),
    false
  )

  // 4. 非法 JSON 自动跳过
  assert.equal(
    chunks.some((c) => c.kb_name === 'DifyKB'),
    false
  )

  // 5. 去重生效（chunk_id=c1 仅一条）
  assert.equal(chunks.filter((c) => c.metadata?.chunk_id === 'c1').length, 1)

  // 6. 分数排序（A 0.9 在 B 0.4 前）
  const idxA = chunks.findIndex((c) => c.content === 'A')
  const idxB = chunks.findIndex((c) => c.content === 'B')
  assert.equal(idxA < idxB, true)

  const conversations = MessageProcessor.convertServerHistoryToMessages([
    { type: 'human', content: '请选择语言' },
    { type: 'ai', content: '请选择输出语言' },
    {
      type: 'human',
      content: '{"language":"python"}',
      extra_metadata: { source: 'ask_user_question_resume' }
    },
    { type: 'ai', content: '这是 Python 版本' }
  ])

  assert.equal(conversations.length, 1)
  assert.equal(conversations[0].messages.length, 3)
  assert.equal(conversations[0].messages.at(-1).content, '这是 Python 版本')
  assert.equal(conversations[0].messages.at(-1).isLast, true)
  assert.equal(conversations[0].status, 'finished')

  const assistantBody = MessageProcessor.parseAssistantMessageBody({
    type: 'ai',
    content: '<think>推理过程</think>最终答案'
  })
  assert.deepEqual(assistantBody, { content: '最终答案', reasoningContent: '推理过程' })

  console.log('messageProcessor extractKnowledgeChunksFromConversation: all assertions passed')
}

run()
