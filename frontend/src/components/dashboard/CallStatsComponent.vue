<template>
  <div class="grid-item call-stats">
    <a-card class="dashboard-card call-stats-section" title="调用统计" :loading="loading">
      <template #extra>
        <div class="simple-controls">
          <div class="simple-toggle-group">
            <span
              v-for="opt in timeRangeOptions"
              :key="opt.value"
              class="simple-toggle"
              :class="{ active: callTimeRange === opt.value }"
              @click="switchTimeRange(opt.value)"
            >{{ opt.label }}
            </span>
          </div>
          <div class="divider"></div>
          <div class="simple-toggle-group">
            <span
              v-for="opt in dataTypeOptions"
              :key="opt.value"
              class="simple-toggle"
              :class="{ active: callDataType === opt.value }"
              @click="switchDataType(opt.value)"
            >{{ opt.label }}
            </span>
          </div>
        </div>
      </template>

      <div class="call-stats-container">
        <div class="chart-container">
          <div ref="callStatsChartRef" class="chart"></div>
        </div>
      </div>
    </a-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import * as echarts from 'echarts'
import { dashboardApi } from '@/apis/dashboard_api'
import { getColorByIndex, truncateLegend } from '@/utils/chartColors'
import { useThemeStore } from '@/stores/theme'

// CSS 变量解析工具函数
function getCSSVariable(variableName, element = document.documentElement) {
  return getComputedStyle(element).getPropertyValue(variableName).trim()
}

const props = defineProps({
  loading: { type: Boolean, default: false }
})

// theme store
const themeStore = useThemeStore()

// state
const callStatsData = ref(null)
const callStatsLoading = ref(false)
const callTimeRange = ref('7days')
const callDataType = ref('tokens')
const timeRangeOptions = [
  { value: 'today', label: '今天' },
  { value: '7days', label: '近7天' },
  { value: 'thisMonth', label: '本月' }
]
const dataTypeOptions = [
  { value: 'models', label: '模型调用' },
  { value: 'agents', label: '智能体调用' },
  { value: 'tokens', label: 'Token消耗' }
]
const isTokenView = computed(() => true)

const formatTokenValue = (value) => {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '0M'
  }
  const millionValue = value / 1_000_000
  const absMillion = Math.abs(millionValue)
  const decimalPlaces = absMillion >= 100 ? 0 : absMillion >= 10 ? 1 : 2
  return `${millionValue.toFixed(decimalPlaces)}M`
}

const formatValueForDisplay = (value) => {
  if (isTokenView.value) {
    return formatTokenValue(value)
  }
  if (typeof value === 'number') {
    return value.toLocaleString()
  }
  return (value ?? 0).toString()
}

const switchTimeRange = (val) => {
  if (callTimeRange.value === val) return
  callTimeRange.value = val
  loadCallStats()
}

const switchDataType = (val) => {
  if (callDataType.value === val) return
  callDataType.value = val
  loadCallStats()
}
const callStatsChartRef = ref(null)
let callStatsChart = null
let retryTimer = null
let hoveredSeriesName = null
let resizeObserver = null
const retryCount = ref(0)
const maxRetry = 20

const loadCallStats = async () => {
  callStatsLoading.value = true
  try {
    const response = await dashboardApi.getCallTimeseries(callDataType.value, callTimeRange.value)
    // Go 后端返回 {code, message, data} 信封，需解包
    callStatsData.value = response.data || response
    await nextTick()
    renderCallStatsChart()
  } catch (error) {
    console.error('加载调用统计数据失败:', error)
  } finally {
    callStatsLoading.value = false
  }
}

const renderCallStatsChart = () => {
  const container = callStatsChartRef.value
  if (!container || !callStatsData.value) return

  if (props.loading) {
    scheduleRetry()
    return
  }

  const { clientWidth, clientHeight } = container
  if (!clientWidth || !clientHeight) {
    scheduleRetry()
    return
  }

  if (retryTimer) {
    clearTimeout(retryTimer)
    retryTimer = null
  }
  retryCount.value = 0

  if (callStatsChart) {
    callStatsChart.dispose()
  }
  hoveredSeriesName = null

  callStatsChart = echarts.init(container)

  // 监听容器尺寸变化，自动调整图表大小
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
  resizeObserver = new ResizeObserver(() => {
    if (callStatsChart) {
      callStatsChart.resize()
    }
  })
  resizeObserver.observe(container)

  const data = callStatsData.value.data || []
  const categories = callStatsData.value.categories || []

  const xAxisData = data.map((item) => {
    const date = item.date
    if (callTimeRange.value === 'today') {
      return date.split(' ')[1]
    } else if (callTimeRange.value === 'thisMonth') {
      return date.split('-').slice(1).join('-')
    } else {
      return date.split('-').slice(1).join('-')
    }
  })

  const agentNames = callStatsData.value.agent_names || {}

  const resolveCategoryLabel = (cat) => {
    if (cat === 'None') return '未知模型'
    return agentNames[cat] || cat
  }

  let series
  if (callDataType.value === 'tokens') {
    // 总Token统计：所有类别汇总为一条折线
    const totalData = data.map((item) => {
      let total = 0
      categories.forEach((cat) => { total += item.data[cat] || 0 })
      return total
    })
    series = [{
      name: '总Token',
      type: 'line',
      smooth: true,
      data: totalData,
      itemStyle: { color: getColorByIndex(0) }
    }]
  } else {
    series = categories.map((category, index) => ({
      name: resolveCategoryLabel(category),
      type: 'line',
      smooth: true,
      data: data.map((item) => item.data[category] || 0),
      itemStyle: { color: getColorByIndex(index) }
    }))
  }

  const option = {
    grid: {
      left: '3%',
      right: '4%',
      top: '5%',
      bottom: 50,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: xAxisData,
      axisLine: { lineStyle: { color: getCSSVariable('--gray-200') } },
      axisTick: { show: false },
      axisLabel: { color: getCSSVariable('--gray-500'), fontSize: 12 }
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: getCSSVariable('--gray-500'),
        fontSize: 12,
        formatter: (value) => (isTokenView.value ? formatTokenValue(value) : value)
      },
      splitLine: { lineStyle: { color: getCSSVariable('--gray-100') } }
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: getCSSVariable('--gray-0'),
      borderColor: getCSSVariable('--gray-200'),
      borderWidth: 1,
      textStyle: { color: getCSSVariable('--gray-600'), fontSize: 12 },
      formatter: (params) => {
        if (!params?.length) return ''
        const visibleParams = params.filter((param) => Number(param.value) !== 0)
        if (!visibleParams.length) return ''
        let total = 0
        let result = `${visibleParams[0].name}<br/>`
        visibleParams.forEach((param) => {
          total += param.value
          const truncatedName = truncateLegend(param.seriesName)
          const isHovered = param.seriesName === hoveredSeriesName
          const itemStyle = isHovered ? 'font-weight:700;color:var(--gray-900)' : ''
          result += `<span style="display:inline-block;margin-right:5px;border-radius:10px;width:10px;height:10px;background-color:${param.color}"></span>`
          result += `<span style="${itemStyle}">${truncatedName}: ${formatValueForDisplay(param.value)}</span><br/>`
        })
        const labelMap = {
          models: '模型Token消耗',
          agents: '智能体Token消耗',
          tokens: '总Token消耗',
          }
        if (callDataType.value === 'tokens') {
          const grandTotal = formatValueForDisplay(total)
          return `<div style="font-weight:bold;margin-bottom:5px">${labelMap[callDataType.value]}</div><strong>总消耗: ${grandTotal}</strong>`
        }
        const formattedTotal = formatValueForDisplay(total)
        return `<div style="font-weight:bold;margin-bottom:5px">${labelMap[callDataType.value]}</div>${result}<strong>总计: ${formattedTotal}</strong>`
      }
    },
    legend: {
      type: callDataType.value === 'tokens' ? 'plain' : 'scroll',
      data: callDataType.value === 'tokens' ? [series[0]?.name || '总Token'] : categories.map(resolveCategoryLabel),
      bottom: 5,
      textStyle: { color: getCSSVariable('--gray-500'), fontSize: 12 },
      itemWidth: 14,
      itemHeight: 14,
      formatter: (name) => truncateLegend(name),
      pageIconSize: 12,
      pageIconColor: getCSSVariable('--gray-500'),
      pageIconInactiveColor: getCSSVariable('--gray-300'),
      pageTextStyle: { color: getCSSVariable('--gray-500') }
    },
    series
  }

  callStatsChart.setOption(option)
  callStatsChart.on('mouseover', (event) => {
    hoveredSeriesName = event?.seriesName || null
  })
  callStatsChart.on('mouseout', () => {
    hoveredSeriesName = null
  })

  window.addEventListener('resize', handleResize, resizeListenerOptions)
}

const scheduleRetry = () => {
  if (retryTimer) clearTimeout(retryTimer)
  retryTimer = setTimeout(() => {
    // 如果容器仍然没有尺寸，设置 ResizeObserver 等待布局完成
    const container = callStatsChartRef.value
    if (container && (!container.clientWidth || !container.clientHeight)) {
      if (!resizeObserver) {
        resizeObserver = new ResizeObserver(() => {
          if (callStatsData.value && !callStatsChart) {
            renderCallStatsChart()
          } else if (callStatsChart) {
            callStatsChart.resize()
          }
        })
        resizeObserver.observe(container)
      }
      return
    }
    retryCount.value += 1
    renderCallStatsChart()
  }, retryCount.value >= maxRetry ? 500 : 100)
}

const handleResize = () => {
  if (callStatsChart) callStatsChart.resize()
}

const resizeListenerOptions = { passive: true }

const cleanup = () => {
  window.removeEventListener('resize', handleResize, resizeListenerOptions)
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (retryTimer) {
    clearTimeout(retryTimer)
    retryTimer = null
  }
  retryCount.value = 0
  if (callStatsChart) {
    callStatsChart.dispose()
    callStatsChart = null
  }
}

defineExpose({ cleanup })

onMounted(() => {
  loadCallStats()
})

watch(
  () => props.loading,
  (now) => {
    if (!now) {
      if (callStatsData.value) {
        nextTick().then(() => renderCallStatsChart())
      }
    }
  }
)

// 监听主题变化，重新渲染图表
watch(
  () => themeStore.isDark,
  () => {
    if (callStatsData.value && callStatsChart) {
      nextTick(() => {
        renderCallStatsChart()
      })
    }
  }
)

onUnmounted(() => {
  cleanup()
})
</script>

<style scoped lang="less">
/* 复用 dashboard.css 样式：此处仅做最小覆盖以避免重复 */
.call-stats-section {
  background-color: var(--gray-0);
  height: 100%;
  display: flex;
  flex-direction: column;
}

:deep(.ant-card-body) {
  flex: 1;
  display: flex;
  padding: 16px; /* 减少padding从20px到16px */
  overflow-x: hidden; /* 防止横向滚动条 */
}

.call-stats-container {
  height: 100%;
  display: flex;
  flex: 1;
}

.call-stats .chart-container {
  height: 100%;
  flex: 1;
  min-height: 250px;
  padding: 0; /* 移除默认padding */
}

.call-stats .chart {
  height: 100% !important;
  min-height: 250px;
  width: 100%;
  padding: 0;
  border: none;
  background-color: transparent;
}

.simple-controls {
  display: flex;
  align-items: center;
  gap: 16px;
}

.simple-toggle-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.simple-toggle-label {
  font-size: 12px;
  color: var(--gray-500);
  margin-right: 4px;
}

.simple-toggle {
  padding: 4px 8px;
  font-size: 12px;
  color: var(--gray-500);
  cursor: pointer;
  border-radius: 4px;
  transition: all 0.2s ease;
  user-select: none;
}

.simple-toggle:hover {
  background-color: var(--gray-100);
  color: var(--gray-700);
}

.simple-toggle.active {
  background-color: var(--main-600);
  color: var(--gray-0);
}

.divider {
  width: 1px;
  height: 16px;
  background-color: var(--gray-200);
}
</style>