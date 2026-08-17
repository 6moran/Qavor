<template>
  <aside class="workspace-sidebar">
    <section class="sidebar-section">
      <button
        type="button"
        class="workspace-nav-item"
        :class="{ active: activeKey === 'personal' }"
        @click="$emit('select-personal')"
      >
        <FileTypeIcon is-dir folder-variant="personal" :size="18" />
        <span>workspaces</span>
      </button>
    </section>
  </aside>
</template>

<script setup>
import FileTypeIcon from '@/components/common/FileTypeIcon.vue'

defineProps({
  activeKey: { type: String, default: 'personal' }
})

defineEmits(['select-personal'])
</script>

<style scoped lang="less">
.workspace-sidebar {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  padding: 14px 10px;
  border-right: 1px solid var(--gray-100);
  background: var(--gray-0);
  overflow-y: auto;
}

.sidebar-section {
  display: flex;
  flex-direction: column;
}

.workspace-nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 36px;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--gray-600);
  font-size: 14px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  transition:
    background-color 0.2s ease,
    color 0.2s ease,
    border-color 0.2s ease;

  span:first-of-type {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 500;
  }

  &:hover:not(:disabled),
  &.active {
    border-color: transparent;
    color: var(--main-600);
  }
}
</style>
