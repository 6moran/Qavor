<template>
  <div
    class="office-file-preview"
    :class="{ 'is-full-height': fullHeight }"
  >
    <div v-if="loading" class="office-state office-loading">
      <a-spin :spinning="true" tip="正在渲染文档..." />
    </div>

    <div v-if="error" class="office-state office-error">
      <FileWarning :size="32" class="office-error-icon" />
      <p class="office-error-text">{{ error }}</p>
      <p class="office-error-hint">当前文件暂不支持在线预览，请下载后查看</p>
    </div>

    <template v-else>
      <!-- Word 文档 -->
      <div
        v-if="isDocx"
        ref="docxContainer"
        class="office-docx-container"
      ></div>

      <!-- Excel 表格 -->
      <div v-else-if="isXlsx" class="office-xlsx-container">
        <div v-if="sheetNames.length > 1" class="xlsx-toolbar">
          <a-select
            v-model:value="activeSheet"
            size="small"
            class="xlsx-sheet-select"
            :options="sheetOptions"
          />
          <span class="xlsx-sheet-count">{{ activeSheetIndex + 1 }} / {{ sheetNames.length }}</span>
        </div>
        <div v-if="sheetHtml" class="xlsx-table-wrap" v-html="sheetHtml"></div>
        <div v-else class="office-state office-error">
          <p class="office-error-text">未读取到表格数据</p>
        </div>
      </div>

      <!-- PowerPoint 幻灯片 -->
      <div
        v-else-if="isPptx"
        ref="pptxContainer"
        class="office-pptx-container"
      ></div>
    </template>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onErrorCaptured, ref, watch } from 'vue'
import { FileWarning } from 'lucide-vue-next'
import { createPreviewRenderGate } from '@/utils/office_preview_lifecycle'

const props = defineProps({
  file: {
    type: Object,
    default: null
  },
  filePath: {
    type: String,
    default: ''
  },
  fullHeight: {
    type: Boolean,
    default: false
  }
})

const previewType = computed(() => props.file?.previewType || '')
const isDocx = computed(() => previewType.value === 'docx')
const isXlsx = computed(() => previewType.value === 'xlsx')
const isPptx = computed(() => previewType.value === 'pptx')

const loading = ref(false)
const error = ref('')
const docxContainer = ref(null)
const pptxContainer = ref(null)

// 渲染错误兜底：捕获组件子树内的任何渲染错误，展示为错误提示，
// 避免渲染库异常中断整个 Vue 组件树（表现为空白 + 弹窗关不掉）。
onErrorCaptured((err, instance, info) => {
  console.error('Office 预览渲染错误:', err, info)
  if (renderTimeout) {
    clearTimeout(renderTimeout)
    renderTimeout = null
  }
  if (!error.value) {
    error.value = err?.message || String(err) || '文档渲染失败'
  }
  loading.value = false
  return false
})

// Excel 状态
const sheetsData = ref([])
const activeSheet = ref('')
let xlsxModule = null
const sheetNames = computed(() => sheetsData.value.map((s) => s.name))
const activeSheetIndex = computed(() => Math.max(0, sheetNames.value.indexOf(activeSheet.value)))
const sheetOptions = computed(() =>
  sheetNames.value.map((name) => ({ label: name, value: name }))
)
const sheetHtml = computed(
  () => sheetsData.value.find((s) => s.name === activeSheet.value)?.html || ''
)

let pptxViewer = null
let renderTimeout = null
const renderGate = createPreviewRenderGate()

const fetchBlob = async (url) => {
  const response = await fetch(url)
  if (!response.ok) throw new Error(`文件获取失败（HTTP ${response.status}）`)
  return response.blob()
}

const loadXlsxModule = async () => {
  if (xlsxModule) return xlsxModule
  const mod = await import('xlsx')
  xlsxModule = mod.default && mod.default.read ? mod.default : mod
  return xlsxModule
}

const renderDocx = async (runId) => {
  const container = docxContainer.value
  if (!container || !props.file?.previewUrl) return
  const blob = await fetchBlob(props.file.previewUrl)
  if (!renderGate.isCurrent(runId)) return
  const { renderAsync } = await import('docx-preview')
  if (!renderGate.isCurrent(runId)) return

  const renderHost = document.createElement('div')
  container.replaceChildren(renderHost)
  await renderAsync(blob, renderHost, renderHost, {
    className: 'docx',
    inWrapper: true,
    ignoreWidth: false,
    ignoreHeight: false,
    ignoreFonts: false,
    breakPages: true,
    experimental: true
  })
  if (!renderGate.isCurrent(runId)) {
    renderHost.remove()
  }
}

const renderXlsx = async (runId) => {
  const url = props.file?.previewUrl
  if (!url) return
  const response = await fetch(url)
  if (!response.ok) throw new Error(`文件获取失败（HTTP ${response.status}）`)
  const buffer = await response.arrayBuffer()
  if (!renderGate.isCurrent(runId)) return
  const xlsx = await loadXlsxModule()
  if (!renderGate.isCurrent(runId)) return
  if (typeof xlsx.read !== 'function') {
    throw new Error('xlsx 解析库加载失败')
  }
  const utils = xlsx.utils || xlsx.default?.utils
  const wb = xlsx.read(buffer, { type: 'array' })
  const data = (wb.SheetNames || []).map((name) => {
    try {
      const ws = wb.Sheets[name]
      return { name, html: ws && utils?.sheet_to_html ? utils.sheet_to_html(ws) : '' }
    } catch (e) {
      console.error(`Excel Sheet「${name}」渲染失败:`, e)
      return { name, html: '', error: e?.message || String(e) }
    }
  })
  if (!renderGate.isCurrent(runId)) return
  sheetsData.value = data
  activeSheet.value = data[0]?.name || ''
}

const renderPptx = async (runId) => {
  const container = pptxContainer.value
  if (!container || !props.file?.previewUrl) return
  const response = await fetch(props.file.previewUrl)
  if (!response.ok) throw new Error(`文件获取失败（HTTP ${response.status}）`)
  const buffer = await response.arrayBuffer()
  if (!renderGate.isCurrent(runId)) return
  const { init } = await import('pptx-preview')
  if (!renderGate.isCurrent(runId)) return

  const renderHost = document.createElement('div')
  container.replaceChildren(renderHost)
  // 使用固定尺寸，避免容器布局未就绪时 clientWidth/clientHeight 异常导致幻灯片不可见
  const viewer = init(renderHost, { width: 960, height: 540 })
  pptxViewer = viewer
  await viewer.preview(buffer)
  if (!renderGate.isCurrent(runId)) {
    try {
      viewer.destroy?.()
    } finally {
      renderHost.remove()
      if (pptxViewer === viewer) pptxViewer = null
    }
  }
}

const render = async (runId) => {
  if (!renderGate.isCurrent(runId)) return
  loading.value = true
  error.value = ''
  // 渲染超时保护：避免动态 import / 渲染库异常导致永久 loading（表现为空白）
  renderTimeout = setTimeout(() => {
    if (renderGate.isCurrent(runId) && loading.value && !error.value) {
      console.warn('Office 文档渲染超时')
      loading.value = false
      error.value = '文档渲染超时，请重试或下载后查看'
    }
  }, 30000)
  try {
    if (isDocx.value) {
      await renderDocx(runId)
    } else if (isXlsx.value) {
      await renderXlsx(runId)
    } else if (isPptx.value) {
      await renderPptx(runId)
    }
  } catch (e) {
    if (renderGate.isCurrent(runId)) {
      console.error('Office 文档渲染失败:', e)
      error.value = e?.message || String(e)
    }
  } finally {
    if (renderGate.isCurrent(runId)) {
      clearTimeout(renderTimeout)
      renderTimeout = null
      loading.value = false
    }
  }
}

const cleanup = () => {
  renderGate.invalidate()
  loading.value = false
  if (renderTimeout) {
    clearTimeout(renderTimeout)
    renderTimeout = null
  }
  error.value = ''
  sheetsData.value = []
  activeSheet.value = ''
  xlsxModule = null
  try {
    if (pptxViewer?.destroy) pptxViewer.destroy()
  } catch (e) {
    console.warn('pptx viewer 清理失败:', e)
  }
  pptxViewer = null
  if (docxContainer.value) docxContainer.value.innerHTML = ''
  if (pptxContainer.value) pptxContainer.value.innerHTML = ''
}

watch(
  () => [props.file?.previewUrl, props.file?.previewType],
  ([url]) => {
    cleanup()
    if (url) {
      const runId = renderGate.begin()
      nextTick(() => {
        if (!renderGate.isCurrent(runId)) return
        render(runId).catch((e) => {
          if (!renderGate.isCurrent(runId)) return
          console.error('Office 渲染未捕获异常:', e)
          error.value = e?.message || String(e)
          loading.value = false
        })
      })
    }
  },
  { immediate: true }
)

onBeforeUnmount(cleanup)
</script>

<style scoped lang="less">
.office-file-preview {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--gray-50);
}

.office-loading {
  position: absolute;
  inset: 0;
  z-index: 2;
  background: var(--gray-50);
}

.office-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 240px;
  height: 100%;
  padding: 32px;
  text-align: center;
  color: var(--gray-600);
}

.office-error-icon {
  color: var(--gray-400);
}

.office-error-text {
  margin: 0;
  font-size: 14px;
  color: var(--gray-700);
}

.office-error-hint {
  margin: 0;
  font-size: 12px;
  color: var(--gray-500);
}

/* Word：白色纸张效果 */
.office-docx-container {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 16px;
  background: var(--gray-100);

  :deep(.docx-wrapper) {
    background: var(--gray-0);
    box-shadow: 0 2px 12px rgba(15, 23, 42, 0.08);
    border-radius: 4px;
    padding: 24px 40px;
  }
}

/* Excel */
.office-xlsx-container {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--gray-0);
}

.xlsx-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--gray-200);
  background: var(--gray-50);
  flex: 0 0 auto;
}

.xlsx-sheet-select {
  width: 220px;
}

.xlsx-sheet-count {
  font-size: 12px;
  color: var(--gray-500);
}

.xlsx-table-wrap {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;

  :deep(table) {
    border-collapse: collapse;
    width: max-content;
    min-width: 100%;
    font-size: 13px;
    font-family: inherit;
    color: var(--gray-900);

    th,
    td {
      border: 1px solid var(--gray-200);
      padding: 6px 10px;
      text-align: left;
      white-space: nowrap;
    }

    th {
      background: var(--gray-100);
      font-weight: 600;
      position: sticky;
      top: 0;
      z-index: 1;
    }

    tr:nth-child(even) td {
      background: var(--gray-25);
    }
  }
}

/* PowerPoint */
.office-pptx-container {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 16px;
  background: var(--gray-100);

  :deep(.pptx-preview) {
    background: var(--gray-0);
  }
}
</style>
