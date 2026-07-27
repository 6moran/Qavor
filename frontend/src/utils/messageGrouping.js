import MessageProcessor from '@/utils/messageProcessor'
import { enrichTaskToolCalls } from '@/components/ToolCallingResult/toolRegistry'

const hasVisibleAssistantBody = (message) => {
  if (!message || message.type !== 'ai') return true

  const { content, reasoningContent } = MessageProcessor.parseAssistantMessageBody(message)
  return Boolean(
    content ||
    reasoningContent ||
    message.error_type ||
    message.extra_metadata?.error_type ||
    message.isStoppedByUser
  )
}

const defaultEnrichToolCalls = (message) => enrichTaskToolCalls(message?.tool_calls)

// 将 AI 消息拆成“正文块”和“工具块”，再跨消息合并相邻工具块。
export const getConversationDisplayItems = (
  conv,
  { enrichToolCalls = defaultEnrichToolCalls } = {}
) => {
  if (!Array.isArray(conv?.messages) || conv.messages.length === 0) return []

  const items = []
  let pendingToolGroup = null

  const flushToolGroup = () => {
    if (pendingToolGroup && pendingToolGroup.toolCalls.length > 0) {
      items.push(pendingToolGroup)
    }
    pendingToolGroup = null
  }

  conv.messages.forEach((message, index) => {
    if (message.type !== 'ai') {
      flushToolGroup()
      items.push({
        type: 'message',
        key: message.id || `message-${index}`,
        message,
        sourceIndex: index
      })
      return
    }

    if (hasVisibleAssistantBody(message)) {
      flushToolGroup()
      items.push({
        type: 'message',
        key: message.id || `message-${index}`,
        message,
        sourceIndex: index
      })
    }

    const toolCalls = enrichToolCalls(message)
    if (toolCalls.length === 0) return

    if (!pendingToolGroup) {
      pendingToolGroup = {
        type: 'tool-group',
        key: `tool-group-${message.id || index}`,
        toolCalls: []
      }
    }
    pendingToolGroup.toolCalls.push(...toolCalls)
  })

  flushToolGroup()
  return items
}
