<template>
  <div class="trace-list-page">
    <a-card :bordered="false">
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
          <a-select-option value="success">成功</a-select-option>
          <a-select-option value="failed">失败</a-select-option>
          <a-select-option value="cancelled">已取消</a-select-option>
          <a-select-option value="timeout">超时</a-select-option>
        </a-select>
        <a-select
          v-model:value="filters.source"
          placeholder="全部来源"
          allow-clear
          style="width: 130px"
        >
          <a-select-option value="sync">同步</a-select-option>
          <a-select-option value="stream">流式</a-select-option>
          <a-select-option value="run">异步 Run</a-select-option>
        </a-select>
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
        @row-click="handleRowClick"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'trace_id'">
            <a-tooltip :title="record.trace_id">
              <span class="trace-id-short">{{ shortId(record.trace_id) }}</span>
            </a-tooltip>
          </template>
          <template v-else-if="column.key === 'status'">
            <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'query'">
            <span class="query-cell">{{ record.query || '-' }}</span>
          </template>
          <template v-else-if="column.key === 'duration'">
            {{ formatDuration(record.duration_ms) }}
          </template>
          <template v-else-if="column.key === 'started_at'">
            {{ formatTime(record.started_at) }}
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
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { traceApi } from '@/apis'
import { agentApi } from '@/apis'

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
  source: undefined,
  range: null
})

const columns = [
  { title: 'TraceID', key: 'trace_id', width: 130 },
  { title: '开始时间', key: 'started_at', width: 170 },
  { title: 'Agent', dataIndex: 'agent_slug', key: 'agent_slug', width: 130 },
  { title: '问题', key: 'query' },
  { title: '模型', dataIndex: 'model_name', key: 'model_name', width: 140 },
  { title: '状态', key: 'status', width: 90 },
  { title: '耗时', key: 'duration', width: 100 },
  { title: 'Token', dataIndex: 'total_tokens', key: 'total_tokens', width: 90 }
]

const statusMap = {
  running: { color: 'processing', label: '运行中' },
  success: { color: 'success', label: '成功' },
  failed: { color: 'error', label: '失败' },
  cancelled: { color: 'default', label: '已取消' },
  timeout: { color: 'warning', label: '超时' }
}

const statusColor = s => statusMap[s]?.color || 'default'
const statusLabel = s => statusMap[s]?.label || s

const shortId = id => (id ? `${id.slice(0, 8)}…` : '-')

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
      source: filters.source || undefined
    }
    if (filters.range && filters.range.length === 2) {
      params.from = filters.range[0].toISOString()
      params.to = filters.range[1].toISOString()
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
  filters.source = undefined
  filters.range = null
  page.value = 1
  loadTraces()
}

const handleRowClick = record => {
  router.push(`/traces/${record.trace_id}`)
}

onMounted(() => {
  loadAgents()
  loadTraces()
})
</script>

<style scoped>
.trace-list-page {
  padding: 16px;
}

.filter-bar {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.trace-id-short {
  font-family: 'JetBrains Mono', Consolas, monospace;
  cursor: pointer;
}

.query-cell {
  display: inline-block;
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
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
