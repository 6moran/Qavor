<template>
  <div class="mcp-cards-page extension-page-root">
    <PageShoulder search-placeholder="搜索 MCP..." v-model:search="searchQuery">
      <template #filters>
        <a-select
          v-model:value="filter"
          class="mcp-filter-select"
          :options="filterOptions"
        />
      </template>
      <template #actions>
        <a-button type="primary" @click="handleMcpAdd" class="lucide-icon-btn">
          <Plus :size="14" />
          <span>添加 MCP</span>
        </a-button>
        <a-tooltip title="刷新 MCP" placement="bottom">
          <a-button class="lucide-icon-btn" :disabled="loading" @click="fetchServers">
            <RefreshCw :size="14" />
          </a-button>
        </a-tooltip>
      </template>
    </PageShoulder>

    <div v-if="visibleServers.length === 0" class="extension-card-grid-empty-state">
      <a-empty
        :image="false"
        :description="searchQuery ? '无匹配 MCP' : filter !== 'all' ? '该筛选下暂无 MCP' : '暂无 MCP，点击上方按钮添加'"
      />
    </div>

    <ExtensionCardGrid v-else :min-width="300">
      <InfoCard
        v-for="server in visibleServers"
        :key="server.name"
        variant="mini"
        :title="formatExtensionCardTitle(server.name)"
        :description="server.description || '暂无描述'"
        :default-icon="Cable"
        :status="cardStatusOf(server)"
        @click="handleCardClick(server)"
      >
        <template #action>
          <button
            type="button"
            class="mcp-card-action"
            :disabled="isActionLoading(server)"
            :aria-label="server.enabled ? '禁用 MCP' : '启用 MCP'"
            @click.stop="server.enabled ? handleSetServerEnabled(server, false) : handleSetServerEnabled(server, true)"
          >
            <Minus v-if="server.enabled" :size="15" class="action-icon action-icon--disable" />
            <Plus v-else :size="15" class="action-icon action-icon--enable" />
          </button>
        </template>
      </InfoCard>
    </ExtensionCardGrid>

    <a-modal
      v-model:open="basicInfoVisible"
      class="mcp-basic-info-modal"
      :footer="null"
      width="560px"
      :destroy-on-close="true"
      @cancel="closeBasicInfo"
    >
      <div v-if="previewServer" class="mcp-basic-info-panel">
        <div class="mcp-basic-info-header">
          <div class="mcp-basic-info-title-area">
            <div class="mcp-basic-info-title">
              {{ formatExtensionCardTitle(previewServer.name) }}
            </div>
            <div class="mcp-basic-info-meta">
              <span>{{ previewServer.transport || '未知传输类型' }}</span>
            </div>
          </div>
        </div>

        <div class="mcp-basic-info-body">
          <div class="mcp-basic-info-row">
            <label>描述</label>
            <span>{{ previewServer.description || '暂无描述' }}</span>
          </div>
          <div class="mcp-basic-info-row">
            <label>传输类型</label>
            <span>{{ previewServer.transport || '-' }}</span>
          </div>
        </div>

        <div class="mcp-basic-info-footer">
          <a-button @click="closeBasicInfo">关闭</a-button>
          <a-button
            type="primary"
            class="lucide-icon-btn"
            :loading="isActionLoading(previewServer)"
            @click="handleSetServerEnabled(previewServer, true)"
          >
            <template #icon><Plus :size="14" /></template>
            添加
          </a-button>
        </div>
      </div>
    </a-modal>

    <McpFormModal
      v-model:open="formModalVisible"
      @submitted="handleFormSubmitted"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { Cable, Minus, Plus, RefreshCw } from 'lucide-vue-next'
import { mcpApi } from '@/apis/mcp_api'
import ExtensionCardGrid from './ExtensionCardGrid.vue'
import InfoCard from '@/components/shared/InfoCard.vue'
import PageShoulder from '@/components/shared/PageShoulder.vue'
import McpFormModal from './McpFormModal.vue'
import { formatExtensionCardTitle } from '@/utils/extensionDisplayName'

const router = useRouter()

const loading = ref(false)
const servers = ref([])
const searchQuery = ref('')
const filter = ref('all') // all | enabled | disabled
const formModalVisible = ref(false)
const basicInfoVisible = ref(false)
const previewServer = ref(null)
const actionLoadingName = ref('')

const filterOptions = [
  { label: '全部', value: 'all' },
  { label: '已启用', value: 'enabled' },
  { label: '未启用', value: 'disabled' }
]

const filteredServers = computed(() => {
  const sorted = [...servers.value].sort((a, b) =>
    String(a.name || '').localeCompare(String(b.name || ''), 'zh-Hans-CN', {
      sensitivity: 'base',
      numeric: true
    })
  )
  if (!searchQuery.value) return sorted
  const q = searchQuery.value.toLowerCase()
  return sorted.filter(
    (s) => s.name.toLowerCase().includes(q) || (s.description || '').toLowerCase().includes(q)
  )
})

const visibleServers = computed(() => {
  if (filter.value === 'enabled') return filteredServers.value.filter((item) => !!item.enabled)
  if (filter.value === 'disabled') return filteredServers.value.filter((item) => !item.enabled)
  return filteredServers.value
})

// 卡片状态：未启用固定显示"未启用"，启用显示连接状态
const cardStatusOf = (server) => {
  if (!server.enabled) return { label: '未启用', level: 'info' }
  const map = {
    connected: { label: '已连接', level: 'success' },
    connecting: { label: '连接中', level: 'warning' },
    failed: { label: '连接失败', level: 'error' },
    unknown: { label: '未连接', level: 'error' }
  }
  return map[server?.status] || { label: '未连接', level: 'error' }
}

const navigateToDetail = (server) => {
  router.push({ path: `/tools/mcp/${encodeURIComponent(server.name)}` })
}

const handleCardClick = (server) => {
  if (server.enabled) {
    navigateToDetail(server)
    return
  }
  openBasicInfo(server)
}

const openBasicInfo = (server) => {
  previewServer.value = server
  basicInfoVisible.value = true
}

const closeBasicInfo = () => {
  basicInfoVisible.value = false
  previewServer.value = null
}

const isActionLoading = (server) => actionLoadingName.value === server?.name

const handleMcpAdd = () => {
  formModalVisible.value = true
}

const handleFormSubmitted = async () => {
  formModalVisible.value = false
  await fetchServers()
}

const handleSetServerEnabled = async (server, enabled) => {
  try {
    actionLoadingName.value = server.name
    const result = await mcpApi.updateMcpServerStatus(server.name, enabled)
    if (result.success) {
      message.success(result.message || `MCP 已${enabled ? '添加' : '移除'}`)
      if (enabled) closeBasicInfo()
      await fetchServers()
    } else {
      message.error(result.message || '操作失败')
    }
  } catch (err) {
    message.error(err.message || '操作失败')
  } finally {
    actionLoadingName.value = ''
  }
}

const fetchServers = async () => {
  try {
    loading.value = true
    const result = await mcpApi.getMcpServers()
    if (result.success) {
      servers.value = result.data || []
    }
  } catch (err) {
    message.error(err.message || '获取 MCP 列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchServers()
})

defineExpose({ fetchServers, loading })
</script>

<style lang="less" scoped>
@import '@/assets/css/extensions.less';

.mcp-card-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--gray-150);
  border-radius: 8px;
  background: var(--gray-0);
  color: var(--main-color);
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease,
    color 0.18s ease;

  &:hover,
  &:focus {
    outline: none;
    border-color: var(--main-200);
    background: var(--main-50);
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }
}

.action-icon {
  flex-shrink: 0;

  &--enable {
    color: var(--color-success-700);
  }

  &--disable {
    color: var(--color-error-700);
  }
}

.mcp-filter-select {
  width: 120px;
  flex-shrink: 0;
}

.mcp-basic-info-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.mcp-basic-info-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.mcp-basic-info-title-area {
  min-width: 0;
}

.mcp-basic-info-title {
  overflow: hidden;
  color: var(--gray-900);
  font-size: 16px;
  font-weight: 700;
  line-height: 22px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mcp-basic-info-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 2px;
  color: var(--gray-500);
  font-size: 12px;
  line-height: 18px;
}

.mcp-basic-info-tag {
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 6px;
  border-radius: 999px;
  background: var(--gray-100);
  color: var(--gray-600);
  font-size: 11px;
  font-weight: 600;
}

.mcp-basic-info-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid var(--gray-150);
  border-radius: 12px;
  background: var(--gray-25);
}

.mcp-basic-info-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  gap: 12px;
  color: var(--gray-700);
  font-size: 13px;
  line-height: 20px;

  label {
    color: var(--gray-500);
    font-weight: 600;
  }

  span {
    min-width: 0;
    overflow-wrap: anywhere;
  }
}

.mcp-basic-info-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
