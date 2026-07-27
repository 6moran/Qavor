<template>
  <a-modal
    :open="open"
    :footer="null"
    :width="800"
    :destroyOnClose="true"
    @cancel="$emit('update:open', false)"
  >
    <template #title>
      <div class="subagent-thread-modal-title">
        <FallbackAvatar
          class="subagent-thread-modal-avatar"
          :src="subagentAvatar"
          :default-src="subagentDefaultAvatar"
          :name="modalTitleName"
          :seed="childThreadId || modalTitleName"
          kind="agent"
          :size="28"
          shape="rounded"
          :alt="`${modalTitleName} icon`"
        />
        <span class="subagent-thread-modal-name">{{ modalTitleName }}</span>
      </div>
    </template>
    <div ref="modalBodyRef" class="subagent-thread-modal-body">
      <div ref="modalContentRef" class="subagent-thread-modal-content">
        <div v-if="loading && !hasRenderableMessages" class="subagent-thread-modal-state">
          正在加载子智能体消息...
        </div>
        <div v-else-if="error" class="subagent-thread-modal-state is-error">{{ error }}</div>
        <ThreadMessageList
          v-else
          :messages="displayMessages"
          :ongoing-messages="displayOngoingMessages"
          :is-processing="displayIsStreaming"
        />
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, nextTick, onUnmounted, reactive, ref, watch } from 'vue'
import { agentApi } from '@/apis'
import { MessageProcessor } from '@/utils/messageProcessor'
import ThreadMessageList from '@/components/ThreadMessageList.vue'
import FallbackAvatar from '@/components/common/FallbackAvatar.vue'
import { processRunSseResponse } from '@/composables/useAgentRunStream'
import { useAgentStreamHandler } from '@/composables/useAgentStreamHandler'
import { useStreamSmoother } from '@/composables/useStreamSmoother'
import ScrollController from '@/utils/scrollController'

const props = defineProps({
  open: {
    type: Boolean,
    default: false
  },
  childThreadId: {
    type: String,
    default: ''
  },
  runId: {
    type: String,
    default: ''
  },
  runStatus: {
    type: String,
    default: ''
  },
  subagentName: {
    type: String,
    default: ''
  },
  subagentAvatar: {
    type: String,
    default: ''
  },
  subagentDefaultAvatar: {
    type: String,
    default: ''
  },
  ongoingMessages: {
    type: Array,
    default: () => []
  },
  isStreaming: {
    type: Boolean,
    default: false
  }
})

defineEmits(['update:open'])

const loading = ref(false)
const error = ref('')
const messages = ref([])
const historyRunId = ref('')
const historyRunStatus = ref('')
const activeStreamRunId = ref('')
const streamReplayActive = ref(false)
const modalBodyRef = ref(null)
const modalContentRef = ref(null)
const streamState = reactive({
  threadStates: {}
})
let streamAbortController = null
let modalScrollContainer = null
let modalResizeObserver = null
let scheduledScrollFrame = null

const RUN_TERMINAL_STATUSES = new Set(['completed', 'failed', 'cancelled', 'interrupted'])
const normalizeRunStatus = (status) => String(status || '').trim()
const isTerminalRunStatus = (status) => RUN_TERMINAL_STATUSES.has(normalizeRunStatus(status))

const modalTitleName = computed(() => props.subagentName || '子智能体')
const effectiveRunStatus = computed(() =>
  normalizeRunStatus(historyRunStatus.value || props.runStatus)
)
const getStreamThreadState = (threadId) => {
  if (!threadId) return null
  if (!streamState.threadStates[threadId]) {
    streamState.threadStates[threadId] = {
      isStreaming: false,
      replyLoadingVisible: false,
      pendingRequestId: null,
      pendingInterrupt: null,
      onGoingConv: {
        msgChunks: {},
        currentRequestKey: null,
        currentAssistantKey: null,
        toolCallBuffers: {}
      },
      agentState: null
    }
  }
  return streamState.threadStates[threadId]
}

const streamSmoother = useStreamSmoother({
  getThreadState: getStreamThreadState
})

const resetStreamState = () => {
  streamSmoother.resetThread()
  Object.keys(streamState.threadStates).forEach((threadId) => {
    delete streamState.threadStates[threadId]
  })
}

const { handleStreamChunk } = useAgentStreamHandler({
  getThreadState: getStreamThreadState,
  processApprovalInStream: () => false,
  currentAgentId: ref(''),
  supportsFiles: ref(false),
  streamSmoother
})

const streamedMessages = computed(() => {
  const threadState = props.childThreadId ? getStreamThreadState(props.childThreadId) : null
  if (!threadState?.onGoingConv) return []
  const msgs = Object.values(threadState.onGoingConv.msgChunks)
    .map(MessageProcessor.mergeMessageChunk)
    .filter(Boolean)
  return msgs.length > 0
    ? MessageProcessor.convertToolResultToMessages(msgs).filter((msg) => msg.type !== 'tool')
    : []
})
const hasStreamedMessages = computed(() => streamedMessages.value.length > 0)
const displayOngoingMessages = computed(() => {
  if (isTerminalRunStatus(effectiveRunStatus.value)) return []
  return hasStreamedMessages.value ? streamedMessages.value : props.ongoingMessages
})
const displayMessages = computed(() =>
  hasStreamedMessages.value && !isTerminalRunStatus(effectiveRunStatus.value) ? [] : messages.value
)
const displayIsStreaming = computed(
  () =>
    !isTerminalRunStatus(effectiveRunStatus.value) &&
    (props.isStreaming || streamReplayActive.value)
)
const effectiveRunId = computed(() => props.runId || historyRunId.value)
const hasRenderableMessages = computed(
  () => displayMessages.value.length > 0 || displayOngoingMessages.value.length > 0
)
const scrollController = new ScrollController(() => modalBodyRef.value, {
  threshold: 80,
  scrollDelay: 80
})

const cancelScheduledScroll = () => {
  if (scheduledScrollFrame === null) return
  if (typeof window !== 'undefined' && typeof window.cancelAnimationFrame === 'function') {
    window.cancelAnimationFrame(scheduledScrollFrame)
  } else {
    clearTimeout(scheduledScrollFrame)
  }
  scheduledScrollFrame = null
}

const scheduleScrollToBottom = (force = false, immediate = false) => {
  if (!props.open) return
  cancelScheduledScroll()
  const schedule =
    typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function'
      ? (callback) => window.requestAnimationFrame(callback)
      : (callback) => setTimeout(callback, 16)
  scheduledScrollFrame = schedule(async () => {
    scheduledScrollFrame = null
    if (immediate) {
      await scrollController.scrollToBottomStaticForce()
    } else {
      await scrollController.scrollToBottom(force)
    }
  })
}

const detachModalScrollTracking = () => {
  cancelScheduledScroll()
  if (modalScrollContainer) {
    modalScrollContainer.removeEventListener('scroll', scrollController.handleScroll)
    modalScrollContainer = null
  }
  if (modalResizeObserver) {
    modalResizeObserver.disconnect()
    modalResizeObserver = null
  }
  scrollController.reset()
}

const attachModalScrollTracking = async () => {
  await nextTick()
  const container = modalBodyRef.value
  if (!container || modalScrollContainer === container) return
  detachModalScrollTracking()
  modalScrollContainer = container
  modalScrollContainer.addEventListener('scroll', scrollController.handleScroll, { passive: true })
  if (typeof window !== 'undefined' && window.ResizeObserver && modalContentRef.value) {
    modalResizeObserver = new ResizeObserver(() => {
      if (displayIsStreaming.value) {
        scheduleScrollToBottom()
      }
    })
    modalResizeObserver.observe(modalContentRef.value)
  }
}

// LangChain 内容块数组 → 纯文本（仅保留 text 块）
const flattenContent = (content) => {
  if (typeof content === 'string') return content
  if (Array.isArray(content)) {
    return content
      .filter((block) => block && block.type === 'text')
      .map((block) => block.text || '')
      .join('')
  }
  return content ?? ''
}

const normalizeMessages = (items) =>
  (Array.isArray(items) ? items : []).map((msg) => ({
    ...msg,
    content: flattenContent(msg.content)
  }))

const loadPersistedMessages = async (threadId) => {
  const response = await agentApi.getAgentHistory(threadId)
  messages.value = normalizeMessages(response.history || [])
}

const loadHistory = async (threadId) => {
  if (!threadId) return
  loading.value = true
  error.value = ''
  messages.value = []
  try {
    historyRunStatus.value = normalizeRunStatus(props.runStatus)
    if (isTerminalRunStatus(historyRunStatus.value)) {
      stopRunStream()
      resetStreamState()
      await loadPersistedMessages(threadId)
      await nextTick()
      if (props.open && props.childThreadId === threadId) {
        scheduleScrollToBottom(true, true)
      }
      return
    }

    // 子智能体消息存于 LangGraph checkpoint，需走 state 接口并把 tool 结果嵌入 AI 消息。
    const response = await agentApi.getAgentState(threadId, { includeMessages: true })
    historyRunId.value = response?.subagent_run?.run_id ? String(response.subagent_run.run_id) : ''
    historyRunStatus.value = normalizeRunStatus(response?.subagent_run?.status || props.runStatus)
    if (isTerminalRunStatus(historyRunStatus.value)) {
      stopRunStream()
      resetStreamState()
      await loadPersistedMessages(threadId)
      await nextTick()
      if (props.open && props.childThreadId === threadId) {
        scheduleScrollToBottom(true, true)
      }
      return
    }

    // checkpoint 的 content 可能是 LangChain 内容块数组，扁平成文本供 MarkdownPreview 渲染。
    const normalized = normalizeMessages(response.messages || [])
    messages.value = MessageProcessor.convertToolResultToMessages(normalized)
    await nextTick()
    if (props.open && props.childThreadId === threadId) {
      scheduleScrollToBottom(true, true)
    }
  } catch (e) {
    error.value = '加载子智能体消息失败'
    console.error('Failed to load subagent thread messages:', e)
  } finally {
    loading.value = false
  }
}

const stopRunStream = () => {
  if (streamAbortController) {
    streamAbortController.abort()
    streamAbortController = null
  }
  activeStreamRunId.value = ''
  streamReplayActive.value = false
}

const routeChunkThreadId = (data, payload, chunk) => {
  return (
    data?.thread_id ||
    payload?.thread_id ||
    chunk?.thread_id ||
    chunk?.meta?.thread_id ||
    chunk?.metadata?.thread_id ||
    props.childThreadId
  )
}

const startRunStreamReplay = async (runId) => {
  stopRunStream()
  resetStreamState()
  if (!runId || !props.childThreadId || isTerminalRunStatus(effectiveRunStatus.value)) return

  const controller = new AbortController()
  streamAbortController = controller
  activeStreamRunId.value = runId
  streamReplayActive.value = true
  getStreamThreadState(props.childThreadId).isStreaming = true

  try {
    const response = await agentApi.streamAgentRunEvents(runId, '0-0', {
      signal: controller.signal
    })
    if (!response.ok) {
      throw new Error(`SSE response not ok: ${response.status}`)
    }

    await processRunSseResponse(response, (event, data) => {
      if (!data) return
      const payload = data.payload || {}
      const chunks = Array.isArray(payload.items)
        ? payload.items
        : payload.chunk
          ? [payload.chunk]
          : []

      chunks.forEach((chunk) => {
        const threadId = routeChunkThreadId(data, payload, chunk)
        if (!threadId || threadId !== props.childThreadId) return
        handleStreamChunk(
          {
            ...chunk,
            request_id: chunk.request_id || data.request_id,
            run_id: chunk.run_id || data.run_id || runId,
            thread_id: threadId
          },
          threadId
        )
      })

      if (event === 'end' || event === 'error') {
        const threadState = getStreamThreadState(props.childThreadId)
        if (threadState) {
          threadState.isStreaming = false
          threadState.replyLoadingVisible = false
        }
      }
    })
  } catch (e) {
    if (e?.name !== 'AbortError') {
      console.error('Failed to stream subagent run messages:', e)
    }
  } finally {
    if (!controller.signal.aborted && props.childThreadId) {
      streamSmoother.flushThread(props.childThreadId)
      scheduleScrollToBottom()
    }
    if (streamAbortController === controller) {
      streamAbortController = null
    }
    streamReplayActive.value = false
    const threadState = getStreamThreadState(props.childThreadId)
    if (threadState) {
      threadState.isStreaming = false
      threadState.replyLoadingVisible = false
    }
  }
}

watch(
  () => [props.open, props.childThreadId, props.runStatus],
  ([isOpen, threadId]) => {
    stopRunStream()
    resetStreamState()
    historyRunId.value = ''
    historyRunStatus.value = ''
    if (isOpen && threadId) {
      attachModalScrollTracking()
      scheduleScrollToBottom(true, true)
      loadHistory(threadId)
    } else {
      detachModalScrollTracking()
    }
  },
  { immediate: true }
)

watch(
  () => [props.open, props.childThreadId, effectiveRunId.value, effectiveRunStatus.value],
  ([isOpen, threadId, runId, runStatus]) => {
    if (!isOpen || !threadId || !runId) return
    if (!runStatus || isTerminalRunStatus(runStatus)) {
      stopRunStream()
      return
    }
    if (activeStreamRunId.value === runId && streamAbortController) return
    startRunStreamReplay(runId)
  },
  { immediate: true }
)

watch(
  displayOngoingMessages,
  () => {
    if (displayIsStreaming.value) {
      scheduleScrollToBottom()
    }
  },
  { deep: true, flush: 'post' }
)

onUnmounted(() => {
  stopRunStream()
  resetStreamState()
  detachModalScrollTracking()
})
</script>

<style lang="less" scoped>
.subagent-thread-modal-title {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding-right: 24px;
}

.subagent-thread-modal-avatar {
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  border: 1px solid var(--gray-150);
  border-radius: 7px;
  background: var(--gray-0);
  object-fit: cover;
}

.subagent-thread-modal-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.subagent-thread-modal-body {
  max-height: 70vh;
  overflow-y: auto;
}

.subagent-thread-modal-content {
  min-height: 100%;
}

.subagent-thread-modal-state {
  padding: 32px 0;
  text-align: center;
  color: var(--gray-500);
  font-size: 13px;

  &.is-error {
    color: var(--color-error-600);
  }
}
</style>
