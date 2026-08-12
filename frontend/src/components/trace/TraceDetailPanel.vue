<template>
  <div class="trace-detail-panel">
    <a-alert v-if="loadError" type="error" show-icon :message="loadError" class="load-error" />
    <a-card :bordered="false" :loading="loading" class="detail-card">
      <template v-if="trace">
        <div class="trace-toolbar" role="toolbar" aria-label="Trace 视图工具栏">
          <strong>Trace Explorer</strong>
          <button
            type="button"
            class="trace-mode-pill"
            :class="{ 'trace-mode-pill-active': filterMode === 'errors' }"
            @click="toggleFilter('errors')"
          >只看错误</button>
          <button
            type="button"
            class="trace-mode-pill"
            :class="{ 'trace-mode-pill-active': filterMode === 'critical' }"
            @click="toggleFilter('critical')"
          >关键路径</button>
          <div class="trace-toolbar-spacer"></div>
          <button type="button" class="trace-toolbar-action" @click="expandAll">全部展开</button>
          <button type="button" class="trace-toolbar-action" @click="collapseAll">全部收起</button>
          <span class="trace-toolbar-count">{{ visibleTimelineRows.length }} / {{ spans.length }} 个 Span</span>
        </div>

        <div v-if="diagnostics.length" class="diagnostics-section">
          <div v-for="d in diagnostics" :key="d.code + d.span_id" class="diag-item" :class="`diag-${d.code}`">
            <span class="diag-badge">{{ diagLabel(d.code) }}</span>
            <span class="diag-msg">{{ d.message }}</span>
          </div>
        </div>

        <a-descriptions :column="3" size="small" bordered class="trace-summary">
          <a-descriptions-item label="TraceID" :span="2"><span class="mono">{{ trace.trace_id }}</span></a-descriptions-item>
          <a-descriptions-item label="入口类型">{{ entryLabel(trace.entry_type) }}</a-descriptions-item>
          <a-descriptions-item label="RequestID">{{ trace.request_id || '-' }}</a-descriptions-item>
          <a-descriptions-item label="ConversationID">{{ trace.conversation_id || '-' }}</a-descriptions-item>
          <a-descriptions-item label="创建时间">{{ formatTime(trace.created_at) }}</a-descriptions-item>
          <a-descriptions-item v-if="run" label="RunID" :span="2">
            <span class="mono">{{ run.run_id }}</span>
            <a-tag :color="runStatusColor(run.status)" class="run-status">{{ runStatusLabel(run.status) }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item v-if="run" label="Run 时间">{{ formatTime(run.started_at) }} ~ {{ formatTime(run.finished_at) }}</a-descriptions-item>
          <a-descriptions-item label="问题" :span="3">{{ trace.query_summary || '-' }}</a-descriptions-item>
        </a-descriptions>

        <section class="trace-explorer">
          <div class="trace-timeline">
            <div class="section-heading">
              <div><strong>耗时瀑布</strong><span>点击 Span 可同步定位右侧详情</span></div>
              <span>时间轴</span>
            </div>
            <a-empty v-if="visibleTimelineRows.length === 0" :image="false" description="当前筛选下暂无 Span" />
            <button
              v-for="row in visibleTimelineRows"
              v-else
              :key="row.span_id"
              type="button"
              class="timeline-row"
              :class="{ 'is-selected': selectedSpanId === row.span_id, 'is-error': row.status === 'error' }"
              @click="selectSpan(row.span_id)"
            >
              <span class="timeline-name" :style="{ paddingLeft: compressedIndent(rowDepth(row), 14, 6, 10) + 'px' }">
                <span class="wf-kind-badge" :class="`wf-kind-${row.kind}`">{{ kindLabel(row.kind) }}</span>
                <span class="timeline-name-text">{{ spanName(row) }}</span>
              </span>
              <span class="timeline-track"><span class="wf-bar" :class="[`wf-bar-${row.kind}`, { 'wf-bar-error': row.status === 'error' }]" :style="{ left: row.leftPct + '%', width: row.widthPct + '%' }"></span></span>
              <span class="timeline-duration">{{ formatDuration(row.actualDurationMs) }}</span>
            </button>
          </div>

          <div class="trace-workspace">
            <div class="trace-tree-pane">
              <div class="section-heading"><div><strong>调用树</strong><span>父子调用结构</span></div></div>
              <a-empty v-if="visibleTreeRoots.length === 0" :image="false" description="当前筛选下暂无 Span" />
              <div v-else class="tree-body">
                <TraceTreeNode
                  v-for="node in visibleTreeRoots"
                  :key="node.span_id"
                  :node="node"
                  :expanded-branches="expandedBranches"
                  :selected-span-id="selectedSpanId"
                  :depth="0"
                  @toggle-branch="toggleBranch"
                  @select-span="selectSpan"
                />
              </div>
            </div>

            <aside class="trace-span-detail">
              <template v-if="activeSpan">
                <a-spin v-if="spanDetailLoading" size="small" />
                <div class="span-detail-heading">
                  <div>
                    <span class="wf-kind-badge" :class="`wf-kind-${activeSpan.kind}`">{{ kindLabel(activeSpan.kind) }}</span>
                    <strong>{{ spanName(activeSpan) }}</strong>
                  </div>
                  <a-tag :color="spanStatusColor(activeSpan.status)">{{ spanStatusText(activeSpan) }}</a-tag>
                </div>
                <div class="span-metrics">
                  <div><span>耗时</span><strong>{{ formatDuration(activeSpan.duration_ms) }}</strong></div>
                  <div><span>输入 Token</span><strong>{{ activeSpan.tokens_in ?? 0 }}</strong></div>
                  <div><span>输出 Token</span><strong>{{ activeSpan.tokens_out ?? 0 }}</strong></div>
                  <div><span>推理 Token</span><strong>{{ activeSpan.reasoning_tokens ?? 0 }}</strong></div>
                </div>
                <SpanDetail :span="activeSpan" />
              </template>
              <a-empty v-else :image="false" description="选择一个 Span 查看详情" />
            </aside>
          </div>
        </section>
      </template>
    </a-card>
  </div>
</template>

<script setup>
import { computed, reactive, ref, h, defineComponent, watch } from 'vue'
import { message } from 'ant-design-vue'
import { ChevronDown, ChevronRight } from 'lucide-vue-next'
import { traceApi } from '@/apis'
import {
  buildTraceTree,
  buildTimelineRows,
  collectDiagnostics,
  collectTreeSpanIds,
  compressedIndent,
  normalizeDiagnostics
} from '@/utils/traceViewModel.js'

const props = defineProps({ traceId: { type: String, required: true } })

const loading = ref(false)
const loadError = ref('')
const trace = ref(null)
const run = ref(null)
const spans = ref([])
const backendDiagnostics = ref([])
const expandedBranches = reactive(new Set())
const selectedSpanId = ref('')
const filterMode = ref('all')

const kindLabel = kind => ({ llm: 'LLM', tool: '工具', retriever: '检索', agent: 'Agent', http: 'HTTP', queue: '队列', event: '事件', persistence: '持久化' }[kind] || kind)
const entryLabel = entry => ({ http: 'HTTP', agent: 'Agent' }[entry] || entry || '-')
const spanName = span => span?.display_name || span?.operation || span?.kind || '-'
const formatTime = value => (value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-')
const formatDuration = ms => ms == null ? '-' : ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(2)} s`

const runStatusMap = {
  pending: { color: 'default', label: '排队中' }, running: { color: 'processing', label: '运行中' },
  completed: { color: 'success', label: '已完成' }, failed: { color: 'error', label: '失败' },
  cancelled: { color: 'default', label: '已取消' }, interrupted: { color: 'warning', label: '已中断' }
}
const runStatusColor = status => runStatusMap[status]?.color || 'default'
const runStatusLabel = status => runStatusMap[status]?.label || status || '-'
const spanStatusText = span => ({ running: '运行中', ok: '成功', error: '失败', cancelled: '已取消', interrupted: '已中断', timeout: '超时' }[span?.status] || span?.status || '-')
const spanStatusColor = status => ({ running: 'processing', ok: 'success', error: 'error', cancelled: 'default', interrupted: 'warning', timeout: 'warning' }[status] || 'default')
const diagLabel = code => ({ running_span: '运行中', orphan_span: '孤儿', status_mismatch: '不一致', dropped_data: '丢失', slow_queue: '慢队列' }[code] || code)

const timelineRows = computed(() => buildTimelineRows(spans.value))
const treeRoots = computed(() => buildTraceTree(spans.value))
const diagnostics = computed(() => backendDiagnostics.value.length ? backendDiagnostics.value : collectDiagnostics(spans.value, run.value))
const spanById = computed(() => new Map(spans.value.map(span => [span.span_id, span])))
const selectedSpan = computed(() => spanById.value.get(selectedSpanId.value) || null)

// 详情列表已剥离 attributes 大字段，选中 span 时按需拉取完整详情
const spanDetailLoading = ref(false)
const spanDetailFull = ref(null)
const activeSpan = computed(() => spanDetailFull.value || selectedSpan.value)
watch(selectedSpanId, async (id) => {
  spanDetailFull.value = null
  if (!id) return
  const listSpan = spanById.value.get(id)
  // 列表 span 不含 attributes（undefined），需要调接口补齐
  if (!listSpan || listSpan.attributes === undefined) {
    spanDetailLoading.value = true
    try {
      const full = await traceApi.getSpan(props.traceId, id)
      if (full && full.span_id === id) spanDetailFull.value = full
    } catch {
      // 拉取失败则回退到列表 span（无 attributes）
    } finally {
      spanDetailLoading.value = false
    }
  }
})

const addAncestors = (ids, span) => {
  let current = span
  const visited = new Set()
  while (current && !visited.has(current.span_id)) {
    ids.add(current.span_id)
    visited.add(current.span_id)
    current = spanById.value.get(current.parent_span_id)
  }
}

const criticalSpanIds = computed(() => {
  const ids = new Set()
  let candidates = treeRoots.value
  while (candidates.length) {
    const current = [...candidates].sort((a, b) => (b.duration_ms || 0) - (a.duration_ms || 0))[0]
    ids.add(current.span_id)
    candidates = current.children || []
  }
  return ids
})

const visibleSpanIds = computed(() => {
  if (filterMode.value === 'all') return null
  if (filterMode.value === 'critical') return criticalSpanIds.value
  const ids = new Set()
  spans.value.filter(span => span.status === 'error').forEach(span => addAncestors(ids, span))
  return ids
})

const filterTree = nodes => nodes.flatMap(node => {
  if (visibleSpanIds.value && !visibleSpanIds.value.has(node.span_id)) return []
  return [{ ...node, children: filterTree(node.children || []) }]
})
const visibleTreeRoots = computed(() => filterTree(treeRoots.value))
const visibleTimelineRows = computed(() => visibleSpanIds.value ? timelineRows.value.filter(row => visibleSpanIds.value.has(row.span_id)) : timelineRows.value)

const rowDepth = row => {
  let depth = 0
  let current = spanById.value.get(row.span_id)
  const visited = new Set([row.span_id])
  while (current?.parent_span_id && spanById.value.has(current.parent_span_id) && !visited.has(current.parent_span_id)) {
    visited.add(current.parent_span_id)
    depth++
    current = spanById.value.get(current.parent_span_id)
  }
  return depth
}

const selectSpan = spanId => { selectedSpanId.value = spanId }
const toggleBranch = spanId => expandedBranches.has(spanId) ? expandedBranches.delete(spanId) : expandedBranches.add(spanId)
const expandAll = () => collectTreeSpanIds(visibleTreeRoots.value).forEach(id => expandedBranches.add(id))
const collapseAll = () => expandedBranches.clear()
const toggleFilter = mode => { filterMode.value = filterMode.value === mode ? 'all' : mode }

watch(visibleTimelineRows, rows => {
  if (rows.length && !rows.some(row => row.span_id === selectedSpanId.value)) {
    selectedSpanId.value = rows.find(row => row.status === 'error')?.span_id || rows[0].span_id
  }
}, { immediate: true })

const loadDetail = async () => {
  loading.value = true
  loadError.value = ''
  trace.value = null
  run.value = null
  spans.value = []
  backendDiagnostics.value = []
  expandedBranches.clear()
  selectedSpanId.value = ''
  filterMode.value = 'all'
  try {
    const data = await traceApi.getTrace(props.traceId)
    if (!data) {
      loadError.value = 'Trace 不存在或已被清理'
      message.warning(loadError.value)
      return
    }
    trace.value = data.trace || null
    run.value = data.run || null
    spans.value = data.spans || []
    backendDiagnostics.value = normalizeDiagnostics(data.diagnostics)
    // 调用树默认全部展开（与时间轴全量展示保持一致）；深层缩进由 compressedIndent 压缩防止显示不全
    collectTreeSpanIds(treeRoots.value).forEach(id => expandedBranches.add(id))
    selectedSpanId.value = spans.value.find(span => span.status === 'error')?.span_id || spans.value[0]?.span_id || ''
  } catch (error) {
    loadError.value = error.message || '加载 Trace 详情失败'
    message.error(loadError.value)
  } finally {
    loading.value = false
  }
}

watch(() => props.traceId, traceId => { if (traceId) loadDetail() }, { immediate: true })

const SpanDetail = defineComponent({
  props: { span: { type: Object, required: true } },
  setup(detailProps) {
    const block = (title, content, error = false) => content ? h('div', { class: 'wf-detail-block' }, [
      h('div', { class: 'wf-detail-title' }, title),
      h('pre', { class: ['wf-detail-pre', { 'error-text': error }] }, content)
    ]) : null
    return () => h('div', { class: 'span-detail-content' }, [
      h('dl', { class: 'span-fields' }, [
        h('div', null, [h('dt', null, 'Operation'), h('dd', null, detailProps.span.operation || '-')]),
        h('div', null, [h('dt', null, 'SpanID'), h('dd', { class: 'mono' }, detailProps.span.span_id || '-')]),
        h('div', null, [h('dt', null, 'RunID'), h('dd', { class: 'mono' }, detailProps.span.run_id || '-')]),
        h('div', null, [h('dt', null, '开始时间'), h('dd', null, formatTime(detailProps.span.started_at))]),
        h('div', null, [h('dt', null, '结束时间'), h('dd', null, formatTime(detailProps.span.ended_at))])
      ]),
      block('输入', detailProps.span.input_summary),
      block('输出', detailProps.span.output_summary),
      block('错误', detailProps.span.error_message, true),
      block('Attributes', detailProps.span.attributes ? JSON.stringify(detailProps.span.attributes, null, 2) : '')
    ])
  }
})

const TraceTreeNode = defineComponent({
  name: 'TraceTreeNode',
  props: {
    node: { type: Object, required: true }, expandedBranches: { type: Object, required: true },
    selectedSpanId: { type: String, default: '' }, depth: { type: Number, default: 0 }
  },
  emits: ['toggle-branch', 'select-span'],
  setup(treeProps, { emit }) {
    return () => {
      const hasChildren = treeProps.node.children.length > 0
      const isExpanded = treeProps.expandedBranches.has(treeProps.node.span_id)
      return h('div', { class: 'tree-node' }, [
        h('div', {
          class: ['tree-node-row', { 'tree-node-row--selected': treeProps.selectedSpanId === treeProps.node.span_id, 'tree-node-row--error': treeProps.node.status === 'error' }],
          style: { paddingLeft: `${compressedIndent(treeProps.depth, 18, 6, 8) + 8}px` },
          role: 'button', tabindex: 0,
          onClick: () => emit('select-span', treeProps.node.span_id),
          onKeydown: event => { if (event.key === 'Enter') emit('select-span', treeProps.node.span_id) }
        }, [
          hasChildren ? h('button', {
            class: 'tree-branch-toggle', type: 'button', 'aria-label': isExpanded ? '收起子调用' : '展开子调用',
            onClick: event => { event.stopPropagation(); emit('toggle-branch', treeProps.node.span_id) }
          }, [h(isExpanded ? ChevronDown : ChevronRight, { size: 14 })]) : h('span', { class: 'tree-branch-placeholder' }),
          h('span', { class: ['wf-kind-badge', `wf-kind-${treeProps.node.kind}`] }, kindLabel(treeProps.node.kind)),
          h('span', { class: 'tree-node-name' }, spanName(treeProps.node)),
          treeProps.node.orphan ? h('span', { class: 'tree-orphan-badge' }, '孤儿') : null,
          treeProps.node.status === 'error' ? h('span', { class: 'wf-error-mark' }, '✕') : null,
          h('span', { class: 'wf-duration-text' }, formatDuration(treeProps.node.duration_ms))
        ]),
        isExpanded && hasChildren ? h('div', { class: 'tree-children' }, treeProps.node.children.map(child => h(TraceTreeNode, {
          key: child.span_id, node: child, expandedBranches: treeProps.expandedBranches,
          selectedSpanId: treeProps.selectedSpanId, depth: treeProps.depth + 1,
          onToggleBranch: id => emit('toggle-branch', id), onSelectSpan: id => emit('select-span', id)
        }))) : null
      ])
    }
  }
})
</script>

<style>
.trace-detail-panel { display:flex; flex-direction:column; height:100%; min-height:0; color:#172033; background:#f4f6fa; }
.load-error { margin:0 0 12px; }
.detail-card { flex:1; min-height:0; overflow:auto; border-radius:10px; box-shadow:0 4px 18px rgba(32,55,100,.06); }
.detail-card .ant-card-body { min-height:100%; padding:0; }
.mono { font-family:'JetBrains Mono',Consolas,monospace; word-break:break-all; }
.error-text { color:#cf1322; white-space:pre-wrap; }
.run-status { margin-left:8px; }
.trace-toolbar { position:sticky; top:0; z-index:3; display:flex; align-items:center; gap:8px; min-height:54px; padding:10px 16px; border-bottom:1px solid #e7eaf0; background:#fff; }
.trace-toolbar strong { margin-right:4px; font-size:15px; }
.trace-mode-pill,.trace-toolbar-action { height:30px; padding:0 11px; border:1px solid #d8dce5; border-radius:6px; color:#667085; background:#fff; font-size:12px; cursor:pointer; }
.trace-mode-pill-active { color:#fff; border-color:#1677ff; background:#1677ff; }
.trace-toolbar-spacer { flex:1; }
.trace-toolbar-action:hover { color:#1677ff; border-color:#1677ff; }
.trace-toolbar-count { color:#8b94a8; font-size:12px; white-space:nowrap; }
.diagnostics-section { display:flex; flex-wrap:wrap; gap:7px; padding:12px 16px 0; }
.diag-item { display:flex; align-items:center; gap:7px; padding:5px 9px; border:1px solid #ffe58f; border-radius:6px; background:#fffbe6; font-size:12px; }
.diag-orphan_span { border-color:#ffa39e; background:#fff1f0; }.diag-dropped_data { border-color:#d3adf7; background:#f9f0ff; }.diag-slow_queue { border-color:#87e8de; background:#e6fffb; }
.diag-badge { font-weight:700; }.diag-msg { color:#59647a; }
.trace-summary { margin:16px; }
.trace-explorer { margin:0 16px 16px; overflow:hidden; border:1px solid #dfe3eb; border-radius:10px; background:#fff; }
.section-heading { display:flex; align-items:center; justify-content:space-between; min-height:40px; padding:8px 12px; border-bottom:1px solid #e7eaf0; background:#fbfcfe; font-size:12px; }
.section-heading div { display:flex; align-items:center; gap:9px; }.section-heading strong { color:#283247; font-size:13px; }.section-heading span { color:#8b94a8; }
.trace-timeline { max-height:310px; overflow:auto; border-bottom:1px solid #dfe3eb; }
.timeline-row { display:grid; grid-template-columns:minmax(180px,34%) 1fr 74px; align-items:center; width:100%; min-height:34px; padding:4px 12px; border:0; border-bottom:1px solid #f0f2f5; color:#3e485e; background:#fff; text-align:left; cursor:pointer; }
.timeline-row:hover,.timeline-row.is-selected { background:#edf5ff; }.timeline-row.is-error { color:#8f1d1d; }.timeline-row:last-child { border-bottom:0; }
.timeline-name { display:flex; align-items:center; gap:6px; min-width:0; }.timeline-name-text { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.timeline-track { position:relative; height:12px; overflow:hidden; border-radius:6px; background:#f0f2f5; }.timeline-duration { color:#8b94a8; text-align:right; font-size:12px; }
.wf-bar { position:absolute; top:0; bottom:0; min-width:3px; border-radius:6px; }.wf-bar-llm { background:#1677ff; }.wf-bar-tool { background:#52c41a; }.wf-bar-retriever { background:#722ed1; }.wf-bar-agent { background:#fa8c16; }.wf-bar-http { background:#7b8496; }.wf-bar-queue { background:#13c2c2; }.wf-bar-event { background:#eb2f96; }.wf-bar-persistence { background:#2f54eb; }.wf-bar-error { background:#cf1322; }
.wf-kind-badge { flex:none; min-width:36px; padding:1px 6px; border-radius:4px; color:#fff; font-size:10px; text-align:center; }.wf-kind-llm { background:#1677ff; }.wf-kind-tool { background:#52c41a; }.wf-kind-retriever { background:#722ed1; }.wf-kind-agent { background:#fa8c16; }.wf-kind-http { background:#7b8496; }.wf-kind-queue { background:#13c2c2; }.wf-kind-event { background:#eb2f96; }.wf-kind-persistence { background:#2f54eb; }.wf-kind-context { background:#722ed1; }
.trace-workspace { display:grid; grid-template-columns:minmax(0,1fr) minmax(330px,40%); min-height:390px; }
.trace-tree-pane { min-width:0; border-right:1px solid #e7eaf0; }.tree-body { overflow:auto; }.tree-node { border-bottom:1px solid #eef0f3; }.tree-node:last-child { border-bottom:0; }
.tree-node-row { display:flex; min-width:max-content; align-items:center; gap:6px; min-height:38px; padding:6px 12px 6px 8px; color:#1f2937; cursor:pointer; transition:background .15s ease,box-shadow .15s ease,color .15s ease; }.tree-node-row:hover { background:#edf5ff; }.tree-node-row--error { color:#8f1d1d; background:#fff8f7; }.tree-node-row--selected { color:#0958d9; background:#dbeafe; outline:1px solid #91caff; outline-offset:-1px; box-shadow:inset 3px 0 #1677ff,0 2px 8px rgba(22,119,255,.14); font-weight:600; }
.tree-branch-toggle,.tree-branch-placeholder { display:inline-flex; flex:0 0 20px; align-items:center; justify-content:center; width:20px; height:20px; }.tree-branch-toggle { padding:0; border:1px solid #c9ced8; border-radius:3px; color:#59647a; background:#fff; cursor:pointer; }
.tree-node-name { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.tree-node-kind { flex:none; color:#4b5563; font-size:12px; font-weight:600; }.tree-node-kind::before { display:inline-block; width:7px; height:7px; margin-right:5px; border-radius:50%; background:currentColor; content:''; }.tree-node-kind-llm { color:#1677ff; }.tree-node-kind-tool { color:#52c41a; }.tree-node-kind-retriever { color:#722ed1; }.tree-node-kind-agent { color:#fa8c16; }.tree-node-kind-http { color:#595959; }.tree-node-kind-queue { color:#08979c; }.tree-node-kind-event { color:#c41d7f; }.tree-node-kind-persistence { color:#2f54eb; }
.tree-orphan-badge { padding:0 4px; border:1px solid #ffa39e; border-radius:3px; color:#cf1322; background:#fff1f0; font-size:10px; }.tree-children { border-left:1px solid #dbe2ea; background:#fbfcfe; }.wf-error-mark { color:#cf1322; font-weight:800; }.wf-duration-text { margin-left:auto; color:#9aa2b1; font-size:12px; }
.trace-span-detail { min-width:0; padding:14px; overflow:auto; background:#fbfcfe; }.span-detail-heading { display:flex; align-items:flex-start; justify-content:space-between; gap:12px; margin-bottom:12px; }.span-detail-heading>div { display:flex; align-items:center; gap:8px; min-width:0; }.span-detail-heading strong { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.span-metrics { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:8px; margin-bottom:14px; }.span-metrics>div { padding:9px 10px; border:1px solid #e7eaf0; border-radius:6px; background:#fff; }.span-metrics span { display:block; color:#8b94a8; font-size:11px; }.span-metrics strong { display:block; margin-top:3px; color:#273147; font-size:13px; }
.span-fields { display:grid; gap:7px; margin:0; }.span-fields>div { display:grid; grid-template-columns:82px minmax(0,1fr); gap:8px; font-size:12px; }.span-fields dt { color:#8b94a8; }.span-fields dd { min-width:0; margin:0; color:#3e485e; word-break:break-all; }.wf-detail-block { margin-top:12px; }.wf-detail-title { margin-bottom:5px; color:#59647a; font-size:12px; font-weight:700; }.wf-detail-pre { max-height:220px; margin:0; padding:9px; overflow:auto; border:1px solid #e5e8ee; border-radius:6px; color:#5a6478; background:#fff; font:12px/1.55 Consolas,monospace; white-space:pre-wrap; word-break:break-word; }
@media (max-width:1000px) { .trace-toolbar { flex-wrap:wrap; }.trace-toolbar-spacer { display:none; }.trace-workspace { grid-template-columns:1fr; }.trace-tree-pane { border-right:0; border-bottom:1px solid #e7eaf0; }.trace-span-detail { max-height:none; }.timeline-row { grid-template-columns:minmax(150px,42%) 1fr 64px; } }
</style>
