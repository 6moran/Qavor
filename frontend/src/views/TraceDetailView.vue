<template>
  <div class="trace-detail-page">
    <PageHeader title="链路追踪 / Trace 详情" :show-border="true">
      <template #actions>
        <a-button @click="router.back()">
          <template #icon><ArrowLeft :size="16" /></template>
          返回
        </a-button>
        <a-tag :color="statusColor(trace.status)">{{ statusLabel(trace.status) }}</a-tag>
      </template>
    </PageHeader>

    <a-card :bordered="false" :loading="loading" class="detail-card">
      <template v-if="trace">
        <!-- 头部信息 -->
        <a-descriptions :column="3" size="small" bordered>
          <a-descriptions-item label="TraceID" :span="2">
            <span class="mono">{{ trace.trace_id }}</span>
          </a-descriptions-item>
          <a-descriptions-item label="来源">{{ sourceLabel(trace.source) }}</a-descriptions-item>
          <a-descriptions-item label="Agent">{{ trace.agent_slug || '-' }}</a-descriptions-item>
          <a-descriptions-item label="模型">{{ trace.model_name || '-' }}</a-descriptions-item>
          <a-descriptions-item label="开始时间">{{ formatTime(trace.started_at) }}</a-descriptions-item>
          <a-descriptions-item label="结束时间">{{ formatTime(trace.ended_at) }}</a-descriptions-item>
          <a-descriptions-item label="总耗时">{{ formatDuration(trace.duration_ms) }}</a-descriptions-item>
          <a-descriptions-item label="总 Token">{{ trace.total_tokens ?? 0 }}</a-descriptions-item>
          <a-descriptions-item label="问题" :span="3">{{ trace.query || '-' }}</a-descriptions-item>
          <a-descriptions-item v-if="trace.error_message" label="错误信息" :span="3">
            <span class="error-text">{{ trace.error_message }}</span>
          </a-descriptions-item>
        </a-descriptions>

        <!-- 瀑布图 -->
        <div class="waterfall-section">
          <div class="waterfall-header">
            <span class="wf-name-col">Span（{{ spans.length }}）</span>
            <span class="wf-time-col">时间轴</span>
          </div>
          <div v-if="spans.length === 0" class="waterfall-empty">暂无 span 数据</div>
          <div v-else class="waterfall-body">
            <div
              v-for="row in waterfallRows"
              :key="row.span.span_id"
              class="wf-row"
              :class="{ 'wf-row-expanded': expanded.has(row.span.span_id) }"
              @click="toggleExpand(row.span.span_id)"
            >
              <div class="wf-name-col" :style="{ paddingLeft: row.depth * 18 + 8 + 'px' }">
                <span class="wf-kind-badge" :class="`wf-kind-${row.span.kind}`">
                  {{ kindLabel(row.span.kind) }}
                </span>
                <span class="wf-name">{{ row.span.name || row.span.kind }}</span>
                <span v-if="row.span.status === 'error'" class="wf-error-mark">✕</span>
                <span class="wf-duration-text">{{ formatDuration(row.span.duration_ms) }}</span>
              </div>
              <div class="wf-time-col">
                <div class="wf-track">
                  <div
                    class="wf-bar"
                    :class="`wf-bar-${row.span.kind} ${row.span.status === 'error' ? 'wf-bar-error' : ''}`"
                    :style="{
                      left: row.leftPct + '%',
                      width: row.widthPct + '%'
                    }"
                  ></div>
                </div>
              </div>
              <!-- 展开详情 -->
              <div v-if="expanded.has(row.span.span_id)" class="wf-detail" @click.stop>
                <a-descriptions :column="3" size="small">
                  <a-descriptions-item label="状态">
                    {{ row.span.status === 'success' ? '成功' : row.span.status === 'error' ? '失败' : '运行中' }}
                  </a-descriptions-item>
                  <a-descriptions-item label="输入 Token">{{ row.span.tokens_in ?? 0 }}</a-descriptions-item>
                  <a-descriptions-item label="输出 Token">{{ row.span.tokens_out ?? 0 }}</a-descriptions-item>
                  <a-descriptions-item v-if="row.span.reasoning_tokens" label="推理 Token">
                    {{ row.span.reasoning_tokens }}
                  </a-descriptions-item>
                  <a-descriptions-item label="开始时间">{{ formatTime(row.span.started_at) }}</a-descriptions-item>
                  <a-descriptions-item label="结束时间">{{ formatTime(row.span.ended_at) }}</a-descriptions-item>
                </a-descriptions>
                <div v-if="row.span.input_summary" class="wf-detail-block">
                  <div class="wf-detail-title">输入</div>
                  <pre class="wf-detail-pre">{{ row.span.input_summary }}</pre>
                </div>
                <div v-if="row.span.output_summary" class="wf-detail-block">
                  <div class="wf-detail-title">输出</div>
                  <pre class="wf-detail-pre">{{ row.span.output_summary }}</pre>
                </div>
                <div v-if="row.span.error_message" class="wf-detail-block">
                  <div class="wf-detail-title">错误</div>
                  <pre class="wf-detail-pre error-text">{{ row.span.error_message }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </a-card>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { ArrowLeft } from 'lucide-vue-next'
import { traceApi } from '@/apis'
import PageHeader from '@/components/shared/PageHeader.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const trace = ref(null)
const spans = ref([])
const expanded = reactive(new Set())

const kindLabel = k => ({ llm: 'LLM', tool: '工具', retriever: '检索', agent: 'Agent' }[k] || k)
const sourceLabel = s => ({ sync: '同步', stream: '流式', run: '异步 Run' }[s] || s || '-')

const statusMap = {
  running: { color: 'processing', label: '运行中' },
  success: { color: 'success', label: '成功' },
  failed: { color: 'error', label: '失败' },
  cancelled: { color: 'default', label: '已取消' },
  timeout: { color: 'warning', label: '超时' }
}
const statusColor = s => statusMap[s]?.color || 'default'
const statusLabel = s => statusMap[s]?.label || s

const formatTime = ts => (ts ? new Date(ts).toLocaleString('zh-CN', { hour12: false }) : '-')

const formatDuration = ms => {
  if (ms == null) return '-'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

// —— 瀑布图计算：按 parent_span_id 组装层级，时间轴按全局最小/最大时间归一化 ——
const waterfallRows = computed(() => {
  const list = spans.value
  if (list.length === 0) return []

  const byId = new Map()
  list.forEach(s => byId.set(s.span_id, s))

  // 计算层级（沿 parent 链上溯，缓存结果）
  const depthCache = new Map()
  const calcDepth = spanId => {
    if (depthCache.has(spanId)) return depthCache.get(spanId)
    const span = byId.get(spanId)
    if (!span) return 0
    let depth = 0
    if (span.parent_span_id && byId.has(span.parent_span_id)) {
      depth = calcDepth(span.parent_span_id) + 1
    }
    depthCache.set(spanId, depth)
    return depth
  }

  // 时间轴范围
  let minStart = Infinity
  let maxEnd = -Infinity
  list.forEach(s => {
    const st = new Date(s.started_at).getTime()
    if (!Number.isNaN(st)) minStart = Math.min(minStart, st)
    const en = s.ended_at ? new Date(s.ended_at).getTime() : st
    if (!Number.isNaN(en)) maxEnd = Math.max(maxEnd, en)
  })
  if (minStart === Infinity || maxEnd === -Infinity) {
    minStart = Date.now()
    maxEnd = minStart + 1
  }
  const totalMs = Math.max(maxEnd - minStart, 1)

  return list.map(span => {
    const st = new Date(span.started_at).getTime()
    const en = span.ended_at ? new Date(span.ended_at).getTime() : st
    const startMs = Number.isNaN(st) ? minStart : st
    const endMs = Number.isNaN(en) ? startMs : en
    const leftPct = Math.max(((startMs - minStart) / totalMs) * 100, 0)
    const widthPct = Math.max(((endMs - startMs) / totalMs) * 100, 0.4)
    return { span, depth: calcDepth(span.span_id), leftPct, widthPct }
  })
})

const toggleExpand = spanId => {
  if (expanded.has(spanId)) {
    expanded.delete(spanId)
  } else {
    expanded.add(spanId)
  }
}

const loadDetail = async () => {
  loading.value = true
  try {
    const data = await traceApi.getTrace(route.params.trace_id)
    if (!data) {
      message.warning('Trace 不存在或已被清理')
      return
    }
    trace.value = data.trace || null
    spans.value = data.spans || []
  } catch (e) {
    message.error(e.message || '加载 Trace 详情失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadDetail)
</script>

<style scoped>
.trace-detail-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--gray-0);
}

.detail-card {
  flex: 1;
  margin: 16px;
  overflow-y: auto;
}

.mono {
  font-family: 'JetBrains Mono', Consolas, monospace;
}

.error-text {
  color: #cf1322;
  white-space: pre-wrap;
}

.waterfall-section {
  margin-top: 24px;
}

.waterfall-header {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 6px 6px 0 0;
  font-weight: 600;
  font-size: 13px;
}

.waterfall-empty {
  padding: 32px;
  text-align: center;
  color: #999;
  border: 1px solid #f0f0f0;
  border-top: none;
}

.waterfall-body {
  border: 1px solid #f0f0f0;
  border-top: none;
  border-radius: 0 0 6px 6px;
}

.wf-name-col {
  flex: 0 0 46%;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding-right: 12px;
}

.wf-time-col {
  flex: 1;
  min-width: 0;
}

.wf-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  padding: 6px 12px;
  border-bottom: 1px solid #f5f5f5;
  cursor: pointer;
  transition: background 0.15s;
}

.wf-row:last-child {
  border-bottom: none;
}

.wf-row:hover {
  background: #f5f8ff;
}

.wf-kind-badge {
  flex-shrink: 0;
  font-size: 11px;
  line-height: 16px;
  padding: 0 6px;
  border-radius: 3px;
  color: #fff;
  min-width: 34px;
  text-align: center;
}

.wf-kind-llm {
  background: #1677ff;
}

.wf-kind-tool {
  background: #52c41a;
}

.wf-kind-retriever {
  background: #722ed1;
}

.wf-kind-agent {
  background: #fa8c16;
}

.wf-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.wf-error-mark {
  color: #cf1322;
  font-weight: 700;
}

.wf-duration-text {
  margin-left: auto;
  color: #999;
  font-size: 12px;
  flex-shrink: 0;
}

.wf-track {
  position: relative;
  height: 18px;
  background: #fafafa;
  border-radius: 3px;
  overflow: hidden;
}

.wf-bar {
  position: absolute;
  top: 2px;
  height: 14px;
  border-radius: 3px;
  opacity: 0.85;
  min-width: 2px;
}

.wf-bar-llm {
  background: #1677ff;
}

.wf-bar-tool {
  background: #52c41a;
}

.wf-bar-retriever {
  background: #722ed1;
}

.wf-bar-agent {
  background: #fa8c16;
}

.wf-bar-error {
  background: repeating-linear-gradient(
    45deg,
    #cf1322,
    #cf1322 4px,
    #ff7875 4px,
    #ff7875 8px
  );
}

.wf-detail {
  flex-basis: 100%;
  padding: 10px 12px 6px;
  background: #fafafa;
  border-radius: 4px;
  margin-top: 6px;
}

.wf-detail-block {
  margin-top: 8px;
}

.wf-detail-title {
  font-size: 12px;
  font-weight: 600;
  color: #666;
  margin-bottom: 4px;
}

.wf-detail-pre {
  margin: 0;
  padding: 8px 10px;
  background: #fff;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 240px;
  overflow: auto;
}
</style>
