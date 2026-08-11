<template>
  <div
    class="token-usage-bar"
    :class="[barToneClass, { 'is-expanded': expanded }]"
  >
    <!-- 收起态：单行窄条 -->
    <div v-if="!expanded" class="token-usage-bar__collapsed" @click="toggle">
      <div class="token-usage-bar__collapsed-left">
        <span class="token-usage-bar__percent">{{ percentLabel }}</span>
        <span class="token-usage-bar__divider">·</span>
        <span class="token-usage-bar__count">{{ countLabel }}</span>
      </div>
      <div class="token-usage-bar__collapsed-right">
        <span class="token-usage-bar__remaining">{{ remainingLabel }}</span>
        <DownOutlined :size="14" class="token-usage-bar__chevron is-collapsed" />
      </div>
    </div>

    <!-- 展开态：完整面板 -->
    <div v-else class="token-usage-bar__expanded">
      <div class="token-usage-bar__expanded-head" @click="toggle">
        <div class="token-usage-bar__expanded-title">
          <span>上下文容量</span>
        </div>
        <div class="token-usage-bar__expanded-summary">
          <span class="token-usage-bar__percent">{{ percentLabel }}</span>
          <span class="token-usage-bar__divider">·</span>
          <span>{{ countLabel }}</span>
          <span class="token-usage-bar__divider">·</span>
          <span>{{ remainingLabel }}</span>
          <DownOutlined :size="14" class="token-usage-bar__chevron" />
        </div>
      </div>

      <!-- 分段进度条 -->
      <div class="token-usage-bar__track" aria-label="Token 构成">
        <div
          v-for="segment in barSegments"
          :key="segment.key"
          class="token-usage-bar__segment"
          :class="segment.tone"
          :style="{ width: segment.percent }"
          :title="`${segment.label}: ${segment.valueLabel}`"
        ></div>
      </div>

      <!-- 图例 -->
      <div class="token-usage-bar__legend">
        <span
          v-for="segment in segments"
          :key="segment.key"
          class="token-usage-bar__legend-item"
        >
          <i :class="segment.tone"></i>
          {{ segment.label }} {{ segment.valueLabel }}
        </span>
      </div>

      <!-- 详情注解 -->
      <div class="token-usage-bar__breakdown">
        <div v-if="contextInfo" class="token-usage-bar__breakdown-row">
          <span>滑动窗口上限</span>
          <strong>{{ formatToken(contextWindow) }} Token</strong>
        </div>
        <div v-if="summaryTriggerTokens > 0" class="token-usage-bar__breakdown-row">
          <span>摘要触发阈值</span>
          <strong>{{ formatToken(summaryTriggerTokens) }} Token</strong>
        </div>
        <div v-if="messagesEstimate" class="token-usage-bar__breakdown-row">
          <span>剩余可发送</span>
          <strong>{{ messagesEstimate }} 条消息</strong>
        </div>
      </div>

      <!-- 快捷按钮 -->
      <div class="token-usage-bar__actions">
        <button type="button" class="token-usage-bar__btn" @click="$emit('refresh')">
          <ReloadOutlined :size="13" />
          刷新
        </button>
        <button type="button" class="token-usage-bar__btn token-usage-bar__btn--danger" @click="$emit('clear-context')">
          <DeleteOutlined :size="13" />
          重置对话
        </button>
      </div>
      
      <!-- 操作提示 -->
      <div class="token-usage-bar__hint">
        点击「重置对话」可选择<span class="token-usage-bar__hint-light">清空上下文</span>（保留会话）或<span class="token-usage-bar__hint-danger">彻底销毁</span>（删除会话）
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { DownOutlined, ReloadOutlined, DeleteOutlined } from '@ant-design/icons-vue'

const props = defineProps({
  tokenUsage: {
    type: Object,
    default: null
  },
  autoPoll: {
    type: Boolean,
    default: true
  },
  pollInterval: {
    type: Number,
    default: 5000
  },
  fetchTokenUsage: {
    type: Function,
    default: null
  }
})

const emit = defineEmits(['refresh', 'clear-context', 'update:expanded'])

const expanded = ref(false)

const toggle = () => {
  expanded.value = !expanded.value
  emit('update:expanded', expanded.value)
  // 展开时自动滚动到可视区域，确保完整面板可见
  if (expanded.value) {
    nextTick(() => {
      const el = document.querySelector('.token-usage-bar-wrapper')
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    })
  }
}

// 轮询
let pollTimer = null
const startPolling = () => {
  if (!props.autoPoll || !props.fetchTokenUsage) return
  stopPolling()
  pollTimer = setInterval(() => {
    props.fetchTokenUsage()
  }, props.pollInterval)
}
const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(() => {
  startPolling()
})
onUnmounted(() => {
  stopPolling()
})

watch(
  () => props.autoPoll,
  (v) => {
    if (v) startPolling()
    else stopPolling()
  }
)

// --- 计算逻辑 ---
const toFiniteNumber = (v) => {
  if (v === null || v === undefined) return null
  const n = Number(v)
  if (!Number.isFinite(n)) return null
  return n
}

const formatToken = (n) => {
  const num = Number(n) || 0
  if (num >= 10000) {
    return `${(num / 1000).toFixed(1).replace(/\.0$/, '')}k`
  }
  return String(Math.round(num))
}

const formatTokenCount = (n) => {
  const num = Number(n) || 0
  const UNIT = 1000
  if (num >= UNIT * 10) return `${(num / UNIT).toFixed(0)}k`
  if (num >= UNIT) return `${(num / UNIT).toFixed(1).replace(/\.0$/, '')}k`
  return String(Math.round(num))
}

const usage = computed(() => props.tokenUsage)

const contextWindow = computed(() => toFiniteNumber(usage.value?.context_window) || 0)
const summaryTriggerTokens = computed(() => toFiniteNumber(usage.value?.summary_trigger_tokens) || 0)
const remainingTokens = computed(() => toFiniteNumber(usage.value?.remaining_context_tokens) || 0)

const total = computed(() => toFiniteNumber(usage.value?.llm_input_tokens) || 0)
// 统一用 context_window 作为总额上限，保证 已用 + 剩余 = 总额
const limit = computed(() => {
  const ctxWin = contextWindow.value
  if (ctxWin > 0) return ctxWin
  const summaryTrigger = summaryTriggerTokens.value
  if (summaryTrigger > 0) return summaryTrigger
  return Math.max(total.value, 1)
})

const percent = computed(() => {
  if (limit.value <= 0) return 0
  return Math.max(0, Math.min((total.value / limit.value) * 100, 100))
})

const percentLabel = computed(() => {
  const p = percent.value
  if (p > 0 && p < 1) return '<1%'
  return `${Math.round(p)}%`
})

const countLabel = computed(() => {
  const lim = limit.value
  if (lim > 0) {
    return `${formatTokenCount(total.value)} / ${formatTokenCount(lim)}`
  }
  return formatTokenCount(total.value)
})

const remainingLabel = computed(() => {
  return `剩余 ${formatTokenCount(remainingTokens.value)}`
})

const barToneClass = computed(() => {
  const p = percent.value
  if (p >= 85) return 'is-danger'
  if (p >= 60) return 'is-warning'
  return 'is-normal'
})

// 分段
const segments = computed(() => {
  const u = usage.value
  if (!u) return []

  const summaryTokens = u.summary_active
    ? Math.max(toFiniteNumber(u.summary_message_tokens) || 0, 0)
    : 0
  const llmMessageTokens = Math.max(toFiniteNumber(u.llm_messages_tokens) || 0, 0)
  const hasSplit =
    toFiniteNumber(u.llm_content_message_tokens) !== null ||
    toFiniteNumber(u.llm_tool_message_tokens) !== null
  const contentMessageTokens = hasSplit
    ? Math.max(toFiniteNumber(u.llm_content_message_tokens) || 0, 0)
    : Math.max(llmMessageTokens - summaryTokens, 0)
  const toolMessageTokens = Math.max(toFiniteNumber(u.llm_tool_message_tokens) || 0, 0)
  const systemTokens = Math.max(toFiniteNumber(u.system_tokens) || 0, 0)

  return [
    { key: 'system', label: '系统提示', value: systemTokens, tone: 'is-system' },
    { key: 'content', label: '对话消息', value: contentMessageTokens, tone: 'is-content' },
    { key: 'tool', label: '工具输出', value: toolMessageTokens, tone: 'is-tool' },
    { key: 'summary', label: '摘要', value: summaryTokens, tone: 'is-summary' }
  ].filter((s) => s.value > 0)
})

const barSegments = computed(() => {
  const lim = Math.max(limit.value, 1)
  let remaining = lim
  return segments.value
    .map((s) => {
      const val = Math.min(s.value, Math.max(remaining, 0))
      remaining -= val
      return {
        ...s,
        percent: `${Math.max(0, Math.min((val / lim) * 100, 100)).toFixed(2)}%`,
        valueLabel: formatTokenCount(s.value)
      }
    })
    .filter((s) => s.value > 0)
})

const contextInfo = computed(() => contextWindow.value > 0)

// 剩余容量预估可发送消息条数
const messagesEstimate = computed(() => {
  const remaining = remainingTokens.value
  if (remaining <= 0) return 0
  // 按平均每条消息 500 tokens 估算
  return Math.max(1, Math.floor(remaining / 500))
})
</script>

<style scoped>
.token-usage-bar {
  width: 100%;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px;
  background: var(--bg-elevated, #ffffff);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  overflow: hidden;
}

.token-usage-bar.is-normal {
  border-color: var(--border-color, #e5e7eb);
}

.token-usage-bar.is-warning {
  border-color: #f59e0b;
  background: linear-gradient(180deg, #fffbeb 0%, #ffffff 30%);
}

.token-usage-bar.is-danger {
  border-color: #ef4444;
  background: linear-gradient(180deg, #fef2f2 0%, #ffffff 30%);
  animation: token-usage-bar-pulse 2s ease-in-out infinite;
}

@keyframes token-usage-bar-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(239, 68, 68, 0); }
  50% { box-shadow: 0 0 0 4px rgba(239, 68, 68, 0.15); }
}

/* 收起态 */
.token-usage-bar__collapsed {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 14px;
  cursor: pointer;
  user-select: none;
  font-size: 12px;
  line-height: 1;
}

.token-usage-bar__collapsed:hover {
  background: var(--bg-hover, #f9fafb);
}

.token-usage-bar__collapsed-left,
.token-usage-bar__collapsed-right {
  display: flex;
  align-items: center;
  gap: 6px;
}

.token-usage-bar__percent {
  font-weight: 600;
  color: var(--gray-900, #111827);
}

.token-usage-bar.is-warning .token-usage-bar__percent {
  color: #b45309;
}

.token-usage-bar.is-danger .token-usage-bar__percent {
  color: #dc2626;
}

.token-usage-bar__divider {
  color: var(--gray-300, #d1d5db);
}

.token-usage-bar__count {
  color: var(--gray-600, #4b5563);
}

.token-usage-bar__remaining {
  color: var(--gray-500, #6b7280);
}

.token-usage-bar__chevron {
  color: var(--gray-400, #9ca3af);
  transition: transform 0.2s ease;
}

.token-usage-bar__chevron.is-collapsed {
  transform: rotate(180deg);
}

/* 展开态 */
.token-usage-bar__expanded {
  padding: 14px 16px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.token-usage-bar__expanded-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
}

.token-usage-bar__expanded-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--gray-900, #111827);
}

.token-usage-bar__expanded-summary {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--gray-600, #4b5563);
  margin-left: auto;
}

.token-usage-bar__track {
  display: flex;
  height: 8px;
  border-radius: 4px;
  overflow: hidden;
  background: var(--gray-100, #f3f4f6);
}

.token-usage-bar__segment {
  height: 100%;
  transition: width 0.3s ease;
}

.token-usage-bar__segment.is-system {
  background: #10b981;
}

.token-usage-bar__segment.is-content {
  background: #3b82f6;
}

.token-usage-bar__segment.is-tool {
  background: #8b5cf6;
}

.token-usage-bar__segment.is-summary {
  background: #f59e0b;
}

.token-usage-bar__legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  font-size: 11px;
  color: var(--gray-600, #4b5563);
}

.token-usage-bar__legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.token-usage-bar__legend-item i {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 2px;
}

.token-usage-bar__legend-item i.is-system { background: #10b981; }
.token-usage-bar__legend-item i.is-content { background: #3b82f6; }
.token-usage-bar__legend-item i.is-tool { background: #8b5cf6; }
.token-usage-bar__legend-item i.is-summary { background: #f59e0b; }

.token-usage-bar__breakdown {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding-top: 2px;
  border-top: 1px solid var(--border-color, #e5e7eb);
}

.token-usage-bar__breakdown-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--gray-500, #6b7280);
}

.token-usage-bar__breakdown-row strong {
  color: var(--gray-900, #111827);
  font-weight: 600;
}

.token-usage-bar__actions {
  display: flex;
  gap: 8px;
  padding-top: 2px;
}

.token-usage-bar__btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 6px;
  border: 1px solid var(--border-color, #e5e7eb);
  background: var(--bg-elevated, #ffffff);
  color: var(--gray-700, #374151);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.token-usage-bar__btn:hover {
  background: var(--bg-hover, #f9fafb);
  border-color: var(--gray-300, #d1d5db);
}

.token-usage-bar__btn--danger {
  color: #dc2626;
  border-color: #fecaca;
}

.token-usage-bar__btn--danger:hover {
  background: #fef2f2;
  border-color: #ef4444;
}

.token-usage-bar__btn--primary {
  color: #2563eb;
  border-color: #93c5fd;
}

.token-usage-bar__btn--primary:hover {
  background: #eff6ff;
  border-color: #3b82f6;
}

.token-usage-bar__hint {
  padding-top: 8px;
  font-size: 11px;
  color: var(--gray-500, #6b7280);
  line-height: 1.4;
}

.token-usage-bar__hint-light {
  color: #2563eb;
  font-weight: 500;
}

.token-usage-bar__hint-danger {
  color: #dc2626;
  font-weight: 500;
}
</style>
