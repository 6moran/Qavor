<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { ChevronDown, Edit3, Plus, RotateCcw, Trash2 } from 'lucide-vue-next'

import PageShoulder from '@/components/shared/PageShoulder.vue'
import { modelApi } from '@/apis/model_api'
import {
  buildModelPayload,
  buildModelTestPayload,
  createDefaultModelForm,
  formatModelTestError,
  formatModelTestSuccess,
  isModelConnectionTestSupported,
  modelToForm,
  MODEL_PROTOCOL_OPTIONS,
  resetAdvancedFields
} from '@/utils/modelConfig'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const testResult = ref(null)
const models = ref([])
const total = ref(0)
// statsModels 缓存全量模型（仅用于顶部统计），避免分页后按类型计数只统计当前页
const statsModels = ref([])
const keyword = ref('')
const modelType = ref('')
const page = ref(1)
const pageSize = ref(10)
const showModal = ref(false)
const editingId = ref(null)
const advancedOpen = ref(false)
const form = reactive(createDefaultModelForm())
// suppressTestResultReset 用于在 resetForm 期间暂停 watch，避免重复清空。
const suppressTestResultReset = ref(false)

const typeOptions = [
  { label: '全部类型', value: '' },
  { label: 'Chat', value: 'chat' },
  { label: 'Embedding', value: 'embedding' },
  { label: 'Rerank', value: 'rerank' }
]

const stats = computed(() => ({
  total: total.value,
  enabled: statsModels.value.filter((model) => model.enabled).length,
  chat: statsModels.value.filter((model) => model.model_type === 'chat').length,
  embedding: statsModels.value.filter((model) => model.model_type === 'embedding').length,
  rerank: statsModels.value.filter((model) => model.model_type === 'rerank').length
}))

// 拉取全量模型用于顶部类型统计（不受分页影响）
const loadStats = async () => {
  try {
    const response = await modelApi.list({ page: 1, page_size: 100 })
    const data = response?.data || { total: 0, items: [] }
    statsModels.value = data.items || []
  } catch {
    // 统计失败不影响列表展示
  }
}

const testResultDescription = computed(() => {
  if (!testResult.value) return ''
  if (!testResult.value.success) {
    const { message, detail } = testResult.value
    // detail 为脱敏后的原始错误，换行展示便于排查
    return detail ? `${message}\n${detail}` : message
  }
  let desc = testResult.value.message
  if (form.model_type === 'embedding' && testResult.value.dimension) {
    desc += ` · 向量维度：${testResult.value.dimension}`
  }
  return desc
})

const resetForm = (source = null) => {
  suppressTestResultReset.value = true
  Object.assign(form, source ? modelToForm(source) : createDefaultModelForm())
  advancedOpen.value = false
  testResult.value = null
  // 下一轮事件循环恢复 watch 监听，确保 Object.assign 触发的回调被跳过。
  Promise.resolve().then(() => {
    suppressTestResultReset.value = false
  })
}

const resetAdvanced = () => {
  Object.assign(form, resetAdvancedFields())
}

// 监听连接相关字段变化，编辑后清空已有测试结果，避免结果与表单不一致。
watch(
  () => [form.name, form.protocol, form.base_url, form.api_key, form.timeout, form.model_type],
  () => {
    if (suppressTestResultReset.value) return
    if (testResult.value) testResult.value = null
  }
)

const loadModels = async () => {
  loading.value = true
  try {
    const response = await modelApi.list({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value,
      model_type: modelType.value
    })
    const data = response?.data || { total: 0, items: [] }
    models.value = data.items || []
    total.value = data.total || 0
  } catch (error) {
    message.error(error.message || '加载模型配置失败')
  } finally {
    loading.value = false
  }
}

const searchModels = () => {
  page.value = 1
  loadModels()
}

const openCreate = () => {
  editingId.value = null
  resetForm()
  showModal.value = true
}

const openEdit = (model) => {
  editingId.value = model.id
  resetForm(model)
  showModal.value = true
}

const save = async () => {
  if (!form.name.trim() || !form.protocol.trim() || !form.base_url.trim()) {
    message.warning('请填写模型名称、协议和 Base URL')
    return
  }

  let payload
  try {
    payload = buildModelPayload(form)
  } catch (error) {
    message.error(error.message)
    return
  }

  saving.value = true
  try {
    if (editingId.value) await modelApi.update(editingId.value, payload)
    else await modelApi.create(payload)
    message.success(editingId.value ? '模型配置已保存' : '模型配置已创建')
    showModal.value = false
    await loadModels()
    loadStats()
  } catch (error) {
    message.error(error.message || '保存模型配置失败')
  } finally {
    saving.value = false
  }
}

const testConnection = async () => {
  if (!isModelConnectionTestSupported(form.model_type)) {
    message.warning('暂不支持该模型类型的连接测试')
    return
  }
  if (!form.name.trim() || !form.protocol.trim() || !form.base_url.trim()) {
    message.warning('请填写模型名称、协议和 Base URL')
    return
  }
  if (!editingId.value && !form.api_key.trim()) {
    message.warning('请填写 API Key')
    return
  }

  let payload
  try {
    payload = buildModelTestPayload(form, editingId.value || null)
  } catch (error) {
    message.error(error.message)
    return
  }

  testing.value = true
  try {
    const response = await modelApi.testConnection(payload)
    if (response?.code !== 0) {
      testResult.value = {
        success: false,
        message: response?.message || '连接测试失败',
        detail: response?.detail || ''
      }
      return
    }
    const data = response?.data || {}
    testResult.value = {
      success: true,
      message: formatModelTestSuccess(data),
      dimension: data.dimension || 0
    }
  } catch (error) {
    const { message: friendly, detail } = formatModelTestError(error)
    testResult.value = {
      success: false,
      message: friendly,
      detail
    }
  } finally {
    testing.value = false
  }
}

const remove = (model) => {
  Modal.confirm({
    title: `删除模型配置“${model.name}”？`,
    content: '删除后将无法通过该模型配置发起请求。',
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    async onOk() {
      try {
        await modelApi.remove(model.id)
        message.success('模型配置已删除')
        if (models.value.length === 1 && page.value > 1) page.value -= 1
        await loadModels()
        loadStats()
      } catch (error) {
        message.error(error.message || '删除模型配置失败')
      }
    }
  })
}

const formatDate = (value) => (value ? new Date(value).toLocaleString() : '-')

defineExpose({ loading, stats, loadModels })
onMounted(() => {
  loadModels()
  loadStats()
})
</script>

<template>
  <div class="model-manage-panel">
    <PageShoulder
      v-model:search="keyword"
      search-placeholder="搜索模型名称..."
      @update:search="searchModels"
    >
      <template #actions>
        <a-select
          v-model:value="modelType"
          :options="typeOptions"
          style="width: 140px"
          @change="searchModels"
        />
        <a-button type="primary" @click="openCreate">
          <Plus :size="15" />
          新增模型
        </a-button>
      </template>
    </PageShoulder>

    <div class="model-toolbar">
      <span>共 {{ total }} 个模型配置</span>
      <span>当前页启用 {{ stats.enabled }} 个</span>
    </div>

    <a-spin :spinning="loading">
      <div v-if="!loading && !models.length" class="empty-state">暂无模型配置</div>
      <div v-else class="model-list">
        <div v-for="model in models" :key="model.id" class="model-row">
          <div class="model-main">
            <div class="model-remark">{{ model.remark || '未填写备注' }}</div>
            <div class="model-url">{{ model.base_url }}</div>
          </div>
          <a-tag :color="model.model_type === 'chat' ? 'blue' : model.model_type === 'embedding' ? 'green' : 'orange'">
            {{ model.model_type === 'rerank' ? 'Rerank' : model.model_type }}
          </a-tag>
          <a-tag :color="model.enabled ? 'success' : 'default'">
            {{ model.enabled ? '已启用' : '已禁用' }}
          </a-tag>
          <span class="model-updated">{{ formatDate(model.updated_at) }}</span>
          <div class="model-actions">
            <a-button type="text" title="编辑" @click="openEdit(model)">
              <Edit3 :size="15" />
            </a-button>
            <a-button type="text" danger title="删除" @click="remove(model)">
              <Trash2 :size="15" />
            </a-button>
          </div>
        </div>
      </div>
    </a-spin>

    <a-pagination
      v-if="total"
      v-model:current="page"
      v-model:page-size="pageSize"
      :total="total"
      :show-size-changer="true"
      class="model-pagination"
      @change="loadModels"
      @show-size-change="loadModels"
    />

    <a-modal
      v-model:open="showModal"
      :title="editingId ? '编辑模型配置' : '新增模型配置'"
      :width="680"
    >
      <div class="form-grid">
        <label>
          模型名称 *
          <a-input v-model:value="form.name" placeholder="例如 gpt-4o" />
        </label>
        <label>
          备注
          <a-input v-model:value="form.remark" placeholder="便于识别此模型配置" />
        </label>
        <label v-if="form.model_type !== 'rerank'">
          协议 *
          <a-select v-model:value="form.protocol" :options="MODEL_PROTOCOL_OPTIONS" />
        </label>
        <label v-else>
          协议
          <div class="rerank-protocol">
            <a-tag color="orange">Rerank 固定协议</a-tag>
            <span class="rerank-protocol-hint">Cohere / Jina 风格 HTTP 重排,自动请求 {base_url}/v1/rerank,无需选择</span>
          </div>
        </label>
        <label class="full-width">
          Base URL *
          <a-input
            v-model:value="form.base_url"
            :placeholder="form.model_type === 'rerank'
              ? 'https://api.jina.ai/v1(自动追加 /v1/rerank)或填完整 /rerank 端点'
              : 'https://api.openai.com/v1'"
          />
        </label>
        <label>
          API Key
          <a-input-password v-model:value="form.api_key" placeholder="留空表示不修改" />
        </label>
        <label>
          超时（毫秒）
          <a-input-number v-model:value="form.timeout" :min="1000" :max="300000" style="width: 100%" />
        </label>
        <label>
          模型类型
          <a-select
            v-model:value="form.model_type"
            :options="typeOptions.slice(1)"
            style="width: 100%"
          />
        </label>
        <div class="enabled-field">
          <span>启用模型</span>
          <a-switch v-model:checked="form.enabled" size="small" />
        </div>
        <div class="advanced-section full-width">
          <button class="advanced-toggle" type="button" @click="advancedOpen = !advancedOpen">
            <span class="advanced-title">
              <ChevronDown :size="15" :class="{ rotated: advancedOpen }" />
              高级选项
            </span>
            <span class="advanced-hint">{{ form.model_type === 'rerank' ? '请求头配置' : '请求头与推理参数' }}</span>
          </button>
          <div v-if="advancedOpen" class="advanced-content">
            <div class="advanced-toolbar">
              <span>高级配置</span>
              <a-button type="link" size="small" @click="resetAdvanced">
                <RotateCcw :size="13" />
                重置
              </a-button>
            </div>
            <div class="advanced-heading">
              <span>请求头 JSON</span>
            </div>
            <a-textarea v-model:value="form.headers" :rows="4" />
            <template v-if="form.model_type !== 'rerank'">
              <div class="advanced-heading">
                <span>默认推理参数 JSON</span>
              </div>
              <a-textarea v-model:value="form.params" :rows="8" />
            </template>
          </div>
        </div>
      </div>

      <a-alert
        v-if="testResult"
        class="model-test-result"
        :type="testResult.success ? 'success' : 'error'"
        :message="testResult.success ? '连接测试成功' : '连接测试失败'"
        show-icon
      >
        <template #description>
          <span class="model-test-result-desc">{{ testResultDescription }}</span>
          <div v-if="!testResult.success && testResult.detail" class="model-test-result-detail">
            {{ testResult.detail }}
          </div>
        </template>
      </a-alert>

      <template #footer>
        <a-button @click="showModal = false">取消</a-button>
        <a-button
          :loading="testing"
          :disabled="!isModelConnectionTestSupported(form.model_type)"
          @click="testConnection"
        >
          测试连接
        </a-button>
        <a-button type="primary" :loading="saving" @click="save">保存</a-button>
      </template>
    </a-modal>
  </div>
</template>

<style lang="less" scoped>
.model-manage-panel {
  min-height: 100%;
  padding: 20px 24px 28px;
}

.model-toolbar {
  display: flex;
  gap: 16px;
  margin: 14px 0 10px;
  color: var(--gray-600);
  font-size: 12px;
}

.model-list {
  overflow: hidden;
  border: 1px solid var(--gray-150);
  border-radius: 8px;
  background: var(--gray-0);
}

.model-row {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 68px;
  padding: 12px 16px;
  border-top: 1px solid var(--gray-100);

  &:first-child { border-top: 0; }
}

.model-main { flex: 1; min-width: 0; }
.model-remark { color: var(--gray-900); font-weight: 600; }
.model-url, .model-updated { color: var(--gray-500); font-size: 12px; }
.model-url { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.model-updated { width: 150px; }
.model-actions { display: flex; gap: 2px; }
.model-pagination { margin-top: 16px; text-align: right; }
.empty-state { padding: 64px 0; color: var(--gray-500); text-align: center; }

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;

  label {
    display: flex;
    flex-direction: column;
    gap: 6px;
    color: var(--gray-700);
    font-size: 12px;
    font-weight: 500;
  }

  .full-width { grid-column: 1 / -1; }
}

.enabled-field {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 10px;
  min-height: 32px;
  color: var(--gray-700);
  font-size: 12px;
  font-weight: 500;
}

.rerank-protocol {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-height: 32px;
  justify-content: center;
}

.rerank-protocol-hint {
  color: var(--gray-500);
  font-size: 11px;
  line-height: 1.5;
}

.advanced-section {
  border-top: 1px solid var(--gray-100);
  padding-top: 4px;
}

.advanced-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 8px 0;
  border: 0;
  background: transparent;
  color: var(--gray-700);
  cursor: pointer;
  text-align: left;
}

.advanced-title,
.advanced-heading,
.advanced-hint {
  display: flex;
  align-items: center;
}

.advanced-title { gap: 6px; font-weight: 600; }
.advanced-title svg { transition: transform 0.2s; }
.advanced-title svg.rotated { transform: rotate(180deg); }
.advanced-hint { color: var(--gray-500); font-size: 11px; }
.advanced-content { display: flex; flex-direction: column; gap: 8px; padding: 4px 0 8px; }
.advanced-toolbar { display: flex; align-items: center; justify-content: space-between; color: var(--gray-700); font-size: 12px; font-weight: 600; }
.advanced-heading { color: var(--gray-700); font-size: 12px; }
.advanced-heading .ant-btn { display: inline-flex; align-items: center; gap: 4px; padding-inline: 4px; }

.model-test-hint {
  margin-top: 12px;
  color: var(--gray-500);
  font-size: 12px;
}

.model-test-result {
  margin-top: 12px;
}

.model-test-result-desc {
  word-break: break-word;
  overflow-wrap: anywhere;
  white-space: pre-line;
}

.model-test-result-detail {
  margin-top: 6px;
  color: var(--gray-500);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
  overflow-wrap: anywhere;
}

@media (max-width: 760px) {
  .model-manage-panel { padding: 14px; }
  .model-row { align-items: flex-start; flex-wrap: wrap; }
  .model-updated { width: auto; }
  .form-grid { grid-template-columns: 1fr; }
  .form-grid .full-width { grid-column: auto; }
  .ant-modal-footer {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
  }
}
</style>
