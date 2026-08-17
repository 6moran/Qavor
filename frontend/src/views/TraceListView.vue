<template>
  <div class="trace-list-page">
    <PageHeader title="链路追踪" :show-border="true" />
    <div class="trace-list-content">
      <a-card :bordered="false" class="trace-card">
        <!-- 筛选区 -->
        <div class="filter-bar">
          <a-input
            v-model:value="filters.keyword"
            placeholder="按问题关键词搜索"
            allow-clear
            style="width: 220px"
            @press-enter="handleSearch"
          />
          <a-select
            v-model:value="filters.agent_slug"
            placeholder="全部 Agent"
            allow-clear
            style="width: 160px"
          >
            <a-select-option v-for="ag in agentOptions" :key="ag.value" :value="ag.value">
              {{ ag.label }}
            </a-select-option>
          </a-select>
          <a-select
            v-model:value="filters.status"
            placeholder="全部状态"
            allow-clear
            style="width: 130px"
          >
            <a-select-option value="running">运行中</a-select-option>
            <a-select-option value="ok">成功</a-select-option>
            <a-select-option value="error">失败</a-select-option>
            <a-select-option value="cancelled">已取消</a-select-option>
            <a-select-option value="interrupted">已中断</a-select-option>
            <a-select-option value="timeout">超时</a-select-option>
          </a-select>
          <a-checkbox v-model:checked="filters.error_only">仅错误</a-checkbox>
          <a-checkbox v-model:checked="filters.mismatch_only">仅不一致</a-checkbox>
          <a-range-picker v-model:value="filters.range" style="width: 240px" />
          <a-button type="primary" @click="handleSearch">查询</a-button>
          <a-button @click="handleReset">重置</a-button>
        </div>

        <!-- 表格 -->
        <a-table
        :columns="columns"
        :data-source="items"
        :loading="loading"
        :pagination="false"
        row-key="trace_id"
        size="middle"
        :custom-row="resolveTraceRow"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'started_at'">
            {{ formatTime(record.started_at) }}
          </template>
          <template v-else-if="column.key === 'agent_status'">
            <a-tag :color="statusColor(record.agent_status)">{{ statusLabel(record.agent_status) }}</a-tag>
            <a-tag v-if="record.status_mismatch" color="warning" title="agent.run 状态与 agent_runs.status 不一致">⚠</a-tag>
          </template>
          <template v-else-if="column.key === 'business_run_status'">
            <a-tag :color="runStatusColor(record.business_run_status)">{{ runStatusLabel(record.business_run_status) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'query_summary'">
            <span class="query-cell">{{ record.query_summary || '-' }}</span>
          </template>
          <template v-else-if="column.key === 'duration_ms'">
            {{ formatDuration(record.duration_ms) }}
          </template>
          <template v-else-if="column.key === 'queue_wait_ms'">
            {{ record.queue_wait_ms ? formatDuration(record.queue_wait_ms) : '-' }}
          </template>
          <template v-else-if="column.key === 'calls'">
            <span class="calls-cell">
              <span class="call-badge llm">{{ record.llm_count ?? 0 }}</span>
              <span class="call-badge tool">{{ record.tool_count ?? 0 }}</span>
            </span>
          </template>
          <template v-else-if="column.key === 'first_error'">
            <a-tooltip v-if="record.first_error" :title="record.first_error">
              <span class="error-text-ellipsis">{{ record.first_error }}</span>
            </a-tooltip>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.key === 'action'">
            <a-button type="link" size="small" @click.stop="openTraceDetail(record)">查看详情</a-button>
          </template>
        </template>
      </a-table>

      <!-- 分页 -->
      <div class="pagination-bar">
        <a-pagination
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="total"
          :show-total="totalText => `共 ${totalText} 条`"
          show-size-changer
          @change="loadTraces"
        />
      </div>
      </a-card>
    </div>

  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { traceApi } from '@/apis'
import { agentApi } from '@/apis'
import PageHeader from '@/components/shared/PageHeader.vue'
import dayjs from '@/utils/time'

const router = useRouter()

const loading = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const agentOptions = ref([])

const filters = reactive({
  keyword: '',
  agent_slug: undefined,
  status: undefined,
  error_only: false,
  mismatch_only: false,
  range: null
})

const columns = [
  { title: '开始时间', key: 'started_at', width: 170 },
  { title: 'Agent', dataIndex: 'agent_slug', key: 'agent_slug', width: 120 },
  { title: '问题摘要', key: 'query_summary' },
  { title: 'Agent 状态', key: 'agent_status', width: 110 },
  { title: 'Run 状态', key: 'business_run_status', width: 100 },
  { title: '总耗时', key: 'duration_ms', width: 100 },
  { title: '排队', key: 'queue_wait_ms', width: 90 },
  { title: 'LLM/工具', key: 'calls', width: 100 },
  { title: 'Token', dataIndex: 'total_tokens', key: 'total_tokens', width: 80 },
  { title: '首个错误', key: 'first_error', width: 200 },
  { title: '操作', key: 'action', width: 110, align: 'center' }
]

// agent.run span 状态
const statusMap = {
  running: { color: 'processing', label: '运行中' },
  ok: { color: 'success', label: '成功' },
  error: { color: 'error', label: '失败' },
  cancelled: { color: 'default', label: '已取消' },
  interrupted: { color: 'warning', label: '已中断' },
  timeout: { color: 'warning', label: '超时' }
}
const statusColor = s => statusMap[s]?.color || 'default'
const statusLabel = s => statusMap[s]?.label || s || '-'

// agent_runs.status 业务状态
const runStatusMap = {
  pending: { color: 'default', label: '排队中' },
  running: { color: 'processing', label: '运行中' },
  completed: { color: 'success', label: '已完成' },
  failed: { color: 'error', label: '失败' },
  cancelled: { color: 'default', label: '已取消' },
  interrupted: { color: 'warning', label: '已中断' }
}
const runStatusColor = s => runStatusMap[s]?.color || 'default'
const runStatusLabel = s => runStatusMap[s]?.label || s || '-'

const formatTime = ts => (ts ? new Date(ts).toLocaleString('zh-CN', { hour12: false }) : '-')

const formatDuration = ms => {
  if (ms == null) return '-'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

const loadAgents = async () => {
  try {
    const res = await agentApi.getAgents()
    const list = res?.data || []
    agentOptions.value = list
      .map(a => ({ value: a.slug || a.agent_slug, label: a.name || a.slug || a.agent_slug }))
      .filter(a => a.value)
  } catch {
    agentOptions.value = []
  }
}

const loadTraces = async () => {
  loading.value = true
  try {
    const params = {
      page: page.value,
      page_size: pageSize.value,
      keyword: filters.keyword || undefined,
      agent_slug: filters.agent_slug || undefined,
      status: filters.status || undefined,
      error_only: filters.error_only || undefined,
      mismatch_only: filters.mismatch_only || undefined
    }
    if (filters.range && filters.range.length === 2) {
      // 用本地时区（上海）的当天起止时间，避免 toISOString() 的 UTC 截断导致当天数据被排除
      params.from = dayjs(filters.range[0]).startOf('day').format('YYYY-MM-DDTHH:mm:ssZ')
      params.to = dayjs(filters.range[1]).endOf('day').format('YYYY-MM-DDTHH:mm:ssZ')
    }
    const data = await traceApi.listTraces(params)
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    message.error(e.message || '加载 Trace 列表失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  page.value = 1
  loadTraces()
}

const handleReset = () => {
  filters.keyword = ''
  filters.agent_slug = undefined
  filters.status = undefined
  filters.error_only = false
  filters.mismatch_only = false
  filters.range = null
  page.value = 1
  loadTraces()
}

const openTraceDetail = record => router.push({
  name: 'TraceDetailComp',
  params: { trace_id: record.trace_id }
})

const isInteractiveTarget = event => Boolean(event.target?.closest?.('button,a,input,textarea,select,[role="button"]'))

const resolveTraceRow = record => ({
  tabindex: 0,
  onClick: event => {
    if (isInteractiveTarget(event)) return
    openTraceDetail(record)
  },
  onKeydown: event => {
    if (event.key !== 'Enter' || isInteractiveTarget(event)) return
    event.preventDefault()
    openTraceDetail(record)
  }
})

onMounted(() => {
  loadAgents()
  loadTraces()
})
</script>

<style scoped>
.trace-list-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--gray-0);
}

.trace-list-content {
  flex: 1;
  min-height: 0;
  padding: 16px;
  overflow-y: auto;
}

.trace-card {
  height: 100%;
}

.filter-bar {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
  margin-bottom: 16px;
}

.query-cell {
  display: inline-block;
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
}

.calls-cell {
  display: inline-flex;
  gap: 4px;
}

.call-badge {
  display: inline-block;
  min-width: 22px;
  text-align: center;
  font-size: 12px;
  line-height: 18px;
  padding: 0 4px;
  border-radius: 3px;
  color: #fff;
}

.call-badge.llm {
  background: #1677ff;
}

.call-badge.tool {
  background: #52c41a;
}

.error-text-ellipsis {
  display: inline-block;
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
  color: #cf1322;
}

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

:deep(.ant-table-row) {
  cursor: pointer;
}
</style>
