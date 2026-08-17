<template>
  <div class="status-bar">
    <div class="status-bar-content">
      <!-- 左侧：系统信息 -->
      <div class="status-left">
        <div class="system-info">
          <div class="system-details">
            <div class="system-name">{{ branding.name }}</div>
            <div class="system-subtitle">{{ branding.subtitle }}</div>
          </div>
        </div>
      </div>
      <!-- 右侧：时间 -->
      <div class="status-right">
        <div class="time-info">
          <Clock class="icon" />
          <span class="current-time">{{ currentTime }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useInfoStore } from '@/stores/info'
import { Clock } from 'lucide-vue-next'
import dayjs from '@/utils/time'

const infoStore = useInfoStore()

const currentTime = ref('')

const branding = computed(() => infoStore.branding)

const updateTime = () => {
  const now = dayjs().tz('Asia/Shanghai')
  currentTime.value = now.format('YYYY年MM月DD日 HH:mm:ss')
}

let timeInterval = null

onMounted(async () => {
  updateTime()
  timeInterval = setInterval(updateTime, 1000)
})

onUnmounted(() => {
  if (timeInterval) {
    clearInterval(timeInterval)
  }
})
</script>

<style scoped lang="less">
.status-bar {
  display: flex;
  align-items: center;
  top: 0;
  z-index: 100;
}

.status-bar-content {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px var(--page-padding);
}

.status-left {
  display: flex;
  align-items: center;
  gap: 24px;
}

.system-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.system-details {
  .system-name {
    font-size: 20px;
    font-weight: 600;
    color: var(--gray-900, #111827);
    line-height: 1.4;
  }

  .system-subtitle {
    font-size: 13px;
    color: var(--gray-600, #6b7280);
    line-height: 1.2;
  }
}

.time-info {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  line-height: 1.3;
  color: var(--gray-600, #4b5563);

  .icon {
    width: 15px;
    height: 15px;
    color: var(--gray-600, #6b7280);
  }
}

.current-time {
  font-weight: 500;
  color: var(--gray-900, #111827);
}

// 响应式设计
@media (max-width: 768px) {
  .status-bar {
    height: 44px;
  }

  .status-bar-content {
    padding: 0 16px;
  }

  .system-details {
    .system-name {
      font-size: 10px;
    }

    .system-subtitle {
      display: none;
    }
  }

  .time-info {
    font-size: 11px;

    .icon {
      width: 12px;
      height: 12px;
    }
  }

  .current-time {
    display: none;
  }
}
</style>