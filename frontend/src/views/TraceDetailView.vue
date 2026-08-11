<template>
  <div class="trace-detail-page">
    <div class="trace-detail-top-bar">
      <button class="detail-back-btn" type="button" @click="goBack">
        <ArrowLeft :size="16" />
        <span>返回</span>
      </button>
      <div class="detail-title-area">
        <div class="detail-icon">
          <Activity :size="18" />
        </div>
        <div class="detail-title-text">
          <h2>链路追踪 / 调用详情</h2>
        </div>
      </div>
    </div>
    <main class="trace-detail-content">
      <TraceDetailPanel :trace-id="traceId" />
    </main>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Activity } from 'lucide-vue-next'
import TraceDetailPanel from '@/components/trace/TraceDetailPanel.vue'

const route = useRoute()
const router = useRouter()
const traceId = computed(() => String(route.params.trace_id || ''))
const goBack = () => router.push('/traces')
</script>

<style scoped>
.trace-detail-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: #f4f6fa;
}

.trace-detail-top-bar {
  padding: 7px var(--page-padding, 16px);
  display: flex;
  align-items: center;
  gap: 16px;
  border-bottom: 1px solid var(--gray-100, #e8e8e8);
  flex-shrink: 0;
  background-color: var(--gray-0, #fff);
}

.detail-back-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: none;
  color: var(--gray-500, #8c8c8c);
  font-size: 16px;
  cursor: pointer;
  padding: 4px 8px;
  margin-left: -8px;
  border-radius: 6px;
  transition: all 0.2s;

  &:hover {
    background-color: var(--gray-100, #f5f5f5);
    color: var(--gray-700, #595959);
  }
}

.detail-title-area {
  display: flex;
  align-items: center;
  gap: 10px;
}

.detail-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background-color: var(--primary-50, #e6f4ff);
  color: var(--primary-600, #1677ff);
}

.detail-title-text {
  display: flex;
  flex-direction: column;

  h2 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--gray-800, #262626);
    line-height: 1.4;
  }
}

.trace-detail-content {
  flex: 1;
  min-height: 0;
  padding: 16px;
  overflow: hidden;
}
</style>
