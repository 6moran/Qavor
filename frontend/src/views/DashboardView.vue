<template>
  <div class="dashboard-container">
    <!-- 顶部状态条 -->
    <div class="modern-stats-header">
      <StatusBar />
    </div>

    <!-- 调用统计模块 -->
    <div class="dashboard-grid">
      <CallStatsComponent :loading="loading" ref="callStatsRef" />
    </div>
  </div>
</template>

<script setup>
import { ref, onUnmounted } from 'vue'

// 导入子组件
import StatusBar from '@/components/StatusBar.vue'
import CallStatsComponent from '@/components/dashboard/CallStatsComponent.vue'

// 调用统计子组件引用
const callStatsRef = ref(null)
const loading = ref(false)

// 清理函数
const cleanupCharts = () => {
  if (callStatsRef.value?.cleanup) callStatsRef.value.cleanup()
}

// 组件卸载时清理图表
onUnmounted(() => {
  cleanupCharts()
})
</script>

<style scoped lang="less">
.dashboard-container {
  background-color: var(--gray-25);
  min-height: calc(100vh - 64px);
  overflow-x: hidden;
}

.dashboard-grid {
  padding: var(--page-padding);
  margin-bottom: 24px;
  display: flex;
  flex-direction: column;
  min-height: 400px;
  height: calc(100vh - 64px - 120px);
}

.grid-item.call-stats {
  flex: 1;
  min-height: 350px;
}
</style>
