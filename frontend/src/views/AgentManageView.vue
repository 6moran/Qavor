<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import PageHeader from '@/components/shared/PageHeader.vue'
import AgentManagePanel from '@/components/model-management/AgentManagePanel.vue'
import ModelManagePanel from '@/components/model-management/ModelManagePanel.vue'

const route = useRoute()
const router = useRouter()

const activeTab = ref('agents')
const agentPanelRef = ref(null)
const providerPanelRef = ref(null)

const modelManageTabs = computed(() => {
  return [
    { key: 'agents', label: '智能体' },
    { key: 'providers', label: '模型配置' }
  ]
})

const activePanel = computed(() =>
  activeTab.value === 'providers' ? providerPanelRef.value : agentPanelRef.value
)

const activeLoading = computed(() => activePanel.value?.loading || false)
const activeStats = computed(() => activePanel.value?.stats || {})

const normalizeTab = (tab) => {
  if (tab === 'providers') return 'providers'
  return 'agents'
}

watch(
  () => [route.query.tab],
  ([tab]) => {
    const nextTab = normalizeTab(tab)
    if (activeTab.value !== nextTab) activeTab.value = nextTab
  },
  { immediate: true }
)

watch(activeTab, (tab) => {
  const nextTab = normalizeTab(tab)
  if (nextTab !== tab) {
    activeTab.value = nextTab
    return
  }
  if (route.query.tab === nextTab) return
  router.replace({ query: { ...route.query, tab: nextTab } })
})
</script>

<template>
  <div class="agent-manage-view">
    <PageHeader
      v-model:active-key="activeTab"
      title="智能体管理"
      :tabs="modelManageTabs"
      :loading="activeLoading"
      :show-border="true"
      aria-label="智能体管理视图切换"
    >
      <template #info>
        <div v-if="activeTab === 'agents'" class="summary-strip">
          <span>{{ activeStats.total || 0 }} 个智能体</span>
          <span>{{ activeStats.global || 0 }} 个全局</span>
          <span v-if="activeStats.builtin">{{ activeStats.builtin }} 个内置</span>
          <span>{{ activeStats.manageable || 0 }} 个可管理</span>
        </div>
        <div v-else class="summary-strip">
          <span>{{ activeStats.total || 0 }} 个模型配置</span>
          <span>{{ activeStats.enabled || 0 }} 个启用</span>
          <span>{{ activeStats.chat || 0 }} 个 Chat</span>
          <span>{{ activeStats.embedding || 0 }} 个 Embedding</span>
          <span>{{ activeStats.rerank || 0 }} 个 Rerank</span>
        </div>
      </template>
    </PageHeader>

    <div class="agent-manage-content">
      <div v-show="activeTab === 'agents'" class="tab-panel">
        <AgentManagePanel ref="agentPanelRef" />
      </div>
      <div v-show="activeTab === 'providers'" class="tab-panel">
        <ModelManagePanel ref="providerPanelRef" />
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.agent-manage-view {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  background: var(--gray-0);
  color: var(--gray-1000);
}

.agent-manage-content {
  flex: 1;
  min-height: 0;
  overflow: hidden;

  .tab-panel {
    height: 100%;
    min-height: 0;
    overflow-y: auto;
  }
}

.summary-strip {
  display: flex;
  gap: 8px;

  span {
    padding: 6px 10px;
    border: 1px solid var(--gray-100);
    border-radius: 7px;
    background: var(--gray-10);
    color: var(--gray-700);
    font-size: 14px;
    line-height: 20px;
  }

  .warning-count {
    background: var(--color-warning-50);
    border-color: var(--color-warning-100);
    color: var(--color-warning-700);
  }
}
</style>
