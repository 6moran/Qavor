<template>
  <a-dropdown
    trigger="click"
    :open="dropdownOpen"
    :disabled="props.disabled"
    @open-change="handleOpenChange"
  >
    <div class="agent-select" :class="agentSelectClasses" @click.prevent.stop>
      <FallbackAvatar
        v-if="selectedAgent?.icon"
        class="agent-avatar"
        :src="selectedAgent.icon"
        :name="selectedAgent.name"
        :seed="selectedAgent.slug"
        kind="agent"
        :size="18"
        shape="rounded"
      />
      <span class="agent-text">{{ displayAgentText }}</span>
    </div>
    <template #overlay>
      <div class="agent-dropdown" @click.stop>
        <a-input v-model:value="keyword" placeholder="搜索智能体" allow-clear />
        <a-menu class="scrollable-menu">
          <a-menu-item v-if="loading" key="loading" disabled>加载中...</a-menu-item>
          <a-menu-item v-else-if="!filteredAgents.length" key="empty" disabled>
            暂无可用智能体
          </a-menu-item>
          <a-menu-item
            v-for="agent in filteredAgents"
            v-else
            :key="agent.slug"
            @click="handleSelectAgent(agent.slug)"
          >
            <div class="agent-option">
              <FallbackAvatar
                v-if="agent.icon"
                class="agent-option-avatar"
                :src="agent.icon"
                :name="agent.name"
                :seed="agent.slug"
                kind="agent"
                :size="24"
                shape="rounded"
              />
              <div class="agent-option-info">
                <span class="agent-option-name">{{ agent.name }}</span>
                <span v-if="agent.description" class="agent-option-desc">{{ agent.description }}</span>
              </div>
            </div>
          </a-menu-item>
        </a-menu>
      </div>
    </template>
  </a-dropdown>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useAgentStore } from '@/stores/agent'
import FallbackAvatar from '@/components/common/FallbackAvatar.vue'

const props = defineProps({
  agentSlug: { type: String, default: '' },
  placeholder: { type: String, default: '选择智能体' },
  size: { type: String, default: 'small' },
  disabled: { type: Boolean, default: false }
})

const emit = defineEmits(['select-agent', 'update:agentSlug'])
const agentStore = useAgentStore()
const keyword = ref('')
const dropdownOpen = ref(false)

const selectedAgent = computed(() => {
  if (!props.agentSlug || !agentStore.agents?.length) return null
  return agentStore.agents.find(a => a.slug === props.agentSlug) || null
})

const displayAgentText = computed(() => {
  if (selectedAgent.value) return selectedAgent.value.name
  return props.placeholder
})

const filteredAgents = computed(() => {
  const value = keyword.value.trim().toLowerCase()
  const agents = agentStore.agents || []
  if (!value) return agents
  return agents.filter(a =>
    `${a.name} ${a.slug} ${a.description || ''}`.toLowerCase().includes(value)
  )
})

const agentSelectClasses = computed(() => ({
  'agent-select--middle': props.size === 'middle',
  'agent-select--large': props.size === 'large',
  'agent-select--disabled': props.disabled
}))

const handleOpenChange = (open) => {
  dropdownOpen.value = open
}

const handleSelectAgent = (slug) => {
  emit('select-agent', slug)
  emit('update:agentSlug', slug)
  dropdownOpen.value = false
  keyword.value = ''
}
</script>

<style scoped>
.agent-select {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  color: var(--text-secondary);
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  transition: all 0.2s;
  max-width: 160px;
}

.agent-select:hover {
  background: var(--bg-tertiary);
  border-color: var(--primary-color);
}

.agent-select--middle {
  padding: 4px 12px;
  font-size: 14px;
}

.agent-select--large {
  padding: 6px 16px;
  font-size: 14px;
}

.agent-select--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.agent-avatar {
  flex-shrink: 0;
}

.agent-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-dropdown {
  background: var(--bg-primary);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  padding: 8px;
  min-width: 220px;
  max-height: 320px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.agent-dropdown :deep(.ant-input) {
  margin-bottom: 8px;
}

.scrollable-menu {
  max-height: 260px;
  overflow-y: auto;
}

.agent-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-option-avatar {
  flex-shrink: 0;
}

.agent-option-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.agent-option-name {
  font-size: 13px;
  font-weight: 500;
}

.agent-option-desc {
  font-size: 11px;
  color: var(--text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 160px;
}
</style>
