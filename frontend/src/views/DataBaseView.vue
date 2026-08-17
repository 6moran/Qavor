<template>
  <div class="database-container layout-container">
    <PageHeader
      v-if="!props.embedded"
      title="知识库"
      :active-key="knowledgeActiveView"
      :tabs="knowledgeViewItems"
      :loading="dbState.listLoading"
      :show-border="true"
      aria-label="知识库视图切换"
    />

    <PageShoulder v-model:search="searchQuery" search-placeholder="搜索知识库...">
      <template #actions>
        <a-button
          type="primary"
          class="lucide-icon-btn"
          @click="openNewDatabaseModal"
        >
          <Plus :size="16" /> 新建知识库
        </a-button>
      </template>
    </PageShoulder>

    <a-modal
      :open="state.openNewDatabaseModel"
      title="新建知识库"
      :confirm-loading="dbState.creating"
      @ok="handleCreateDatabase"
      @cancel="cancelCreateDatabase"
      class="new-database-modal"
      width="800px"
      destroyOnClose
    >
      <div class="new-database-form">
        <div class="form-section">
          <h3 class="section-title">知识库名称<span class="required-mark">*</span></h3>
          <a-input v-model:value="newDatabase.name" placeholder="新建知识库名称" />
        </div>

        <div class="form-grid two-columns">
          <div class="form-section compact-section">
            <h3 class="section-title">嵌入模型</h3>
            <a-select
              v-model:value="newDatabase.embedding_model_id"
              :loading="modelsLoading"
              class="full-width"
              placeholder="请选择嵌入模型"
            >
              <a-select-option
                v-for="option in embeddingModelOptions"
                :key="option.id"
                :value="option.value"
              >
                <div class="model-option">
                  <div class="model-option-title">
                    <span>{{ option.label }}</span>
                    <span class="model-option-id">#{{ option.id }}</span>
                  </div>
                  <div class="model-option-remark" :title="option.remark">
                    {{ option.remark }}
                  </div>
                </div>
              </a-select-option>
            </a-select>
          </div>

          <div class="form-section compact-section">
            <h3 class="section-title">问答模型<span class="required-mark">*</span></h3>
            <a-select
              v-model:value="newDatabase.chat_model_id"
              :loading="modelsLoading"
              class="full-width"
              placeholder="请选择问答模型"
            >
              <a-select-option v-for="option in chatModelOptions" :key="option.id" :value="option.value">
                <div class="model-option">
                  <div class="model-option-title">
                    <span>{{ option.label }}</span>
                    <span class="model-option-id">#{{ option.id }}</span>
                  </div>
                  <div class="model-option-remark" :title="option.remark">
                    {{ option.remark }}
                  </div>
                </div>
              </a-select-option>
            </a-select>
          </div>

          <div class="form-section compact-section">
            <h3 class="section-title">Rerank 模型</h3>
            <a-input
              :value="globalRerankModelDisplay"
              :loading="ragSettingsLoading"
              disabled
            />
            <a-button type="link" class="settings-link" @click="openRagSettings">
              前往设置
            </a-button>
          </div>

          <div class="form-section compact-section">
            <div class="chunk-preset-title-row">
              <h3 class="section-title">分块策略</h3>
              <a-tooltip :title="selectedPresetDescription">
                <QuestionCircleOutlined class="chunk-preset-help-icon" />
              </a-tooltip>
            </div>
            <a-select
              v-model:value="newDatabase.chunk_preset_id"
              :options="chunkPresetOptions"
              :loading="chunkPresetLoading"
              class="full-width"
            />
          </div>
        </div>

        <div class="form-section">
          <h3 class="section-title">知识库描述<span class="required-mark">*</span></h3>
          <p class="field-hint description-hint">
            在智能体流程中，这里的描述会作为工具的描述。智能体会根据知识库的标题和描述来选择合适的工具。所以这里描述的越详细，智能体越容易选择到合适的工具。
          </p>
          <AiTextarea
            v-model="newDatabase.description"
            :name="newDatabase.name"
            :chat-model-id="newDatabase.chat_model_id"
            placeholder="新建知识库描述"
            :auto-size="{ minRows: 3, maxRows: 10 }"
          />
        </div>

      </div>
      <template #footer>
        <a-button key="back" @click="cancelCreateDatabase">取消</a-button>
        <a-button
          key="submit"
          type="primary"
          :loading="dbState.creating"
          @click="handleCreateDatabase"
          >创建</a-button
        >
      </template>
    </a-modal>

    <!-- 加载状态 -->
    <div v-if="dbState.listLoading" class="loading-container">
      <a-spin size="large" />
      <p>正在加载知识库...</p>
    </div>

    <!-- 空状态显示 -->
    <ResourceEmptyState
      v-else-if="!databases || databases.length === 0"
      title="暂无知识库"
      description="创建知识库后，可以上传文件并配置检索、图谱和评估能力。"
      :icon="Database"
    >
      <template #actions>
        <a-button
          type="primary"
          size="large"
          class="lucide-icon-btn"
          @click="openNewDatabaseModal"
        >
          <template #icon>
            <Plus :size="16" />
          </template>
          创建知识库
        </a-button>
      </template>
    </ResourceEmptyState>

    <!-- 数据库列表 -->
    <ExtensionCardGrid v-else>
      <InfoCard
        v-for="database in filteredDatabases"
        :key="database.kb_id"
        :title="database.name"
        :subtitle="cardSubtitle(database)"
        :description="database.description || '暂无描述'"
        :tags="cardTags(database)"
        @click="navigateToDatabase(database)"
      >
        <template #icon>
          <Database :size="20" />
        </template>
        <template #card-more-action-corner>
          <a-menu @click="({ key }) => handleDatabaseAction(key, database)">
            <a-menu-item key="copy">
              <span class="lucide-menu-item">
                <Copy :size="15" />
                <span>复制 ID</span>
              </span>
            </a-menu-item>
            <a-menu-item key="edit">
              <span class="lucide-menu-item">
                <Pencil :size="15" />
                <span>编辑知识库</span>
              </span>
            </a-menu-item>
            <a-menu-divider />
            <a-menu-item key="delete" danger>
              <span class="lucide-menu-item">
                <Trash2 :size="15" />
                <span>删除知识库</span>
              </span>
            </a-menu-item>
          </a-menu>
        </template>
      </InfoCard>
    </ExtensionCardGrid>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive, watch, computed, inject } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useDatabaseStore } from '@/stores/database'
import { useConfigStore } from '@/stores/config'
import { QuestionCircleOutlined } from '@ant-design/icons-vue'
import { Copy, Database, Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { message, Modal } from 'ant-design-vue'
import { databaseApi, modelApi } from '@/apis/knowledge_api'
import PageHeader from '@/components/shared/PageHeader.vue'
import PageShoulder from '@/components/shared/PageShoulder.vue'
import ResourceEmptyState from '@/components/shared/ResourceEmptyState.vue'
import ExtensionCardGrid from '@/components/extensions/ExtensionCardGrid.vue'
import InfoCard from '@/components/shared/InfoCard.vue'
import dayjs, { parseToShanghai } from '@/utils/time'
import AiTextarea from '@/components/AiTextarea.vue'
import { useChunkPresetOptions } from '@/composables/useChunkPresetOptions'
import { DEFAULT_CHUNK_PRESET_ID } from '@/utils/chunkUtils'
import { buildKnowledgeBaseCreateRequest } from '@/utils/knowledge_base_create'
import { buildModelSelectOptions } from '@/utils/model_options'

const route = useRoute()
const router = useRouter()
const databaseStore = useDatabaseStore()
const configStore = useConfigStore()
const { openSettingsModal } = inject('settingsModal', {})
const {
  chunkPresetSelectOptions: chunkPresetOptions,
  chunkPresetLoading,
  loadChunkPresetOptions,
  getChunkPresetDescription
} = useChunkPresetOptions()

const props = defineProps({
  embedded: { type: Boolean, default: false }
})

// 使用 store 的状态
const { databases, state: dbState } = storeToRefs(databaseStore)

const knowledgeActiveView = 'documents'
const knowledgeViewItems = [
  { key: 'documents', label: '文档知识库', path: '/extensions?tab=knowledge' }
]

const searchQuery = ref('')

const filteredDatabases = computed(() => {
  let list = databases.value
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(
      (db) =>
        db.name.toLowerCase().includes(q) ||
        (db.description && db.description.toLowerCase().includes(q))
    )
  }
  return list
})

const state = reactive({
  openNewDatabaseModel: false
})

const embeddingModels = ref([])
const chatModels = ref([])
const modelsLoading = ref(false)
const ragSettingsLoading = ref(false)
const globalRerankModelDisplay = computed(
  () => configStore.ragSettings.rerankModelName || '未配置，将使用 RRF 融合结果'
)
const embeddingModelOptions = computed(() =>
  buildModelSelectOptions(embeddingModels.value)
)
const chatModelOptions = computed(() =>
  buildModelSelectOptions(chatModels.value)
)

const createEmptyDatabaseForm = () => ({
  name: '',
  description: '',
  embedding_model_id: undefined,
  chat_model_id: undefined,
  chunk_preset_id: DEFAULT_CHUNK_PRESET_ID
})

const newDatabase = reactive(createEmptyDatabaseForm())

const selectedPresetDescription = computed(() =>
  getChunkPresetDescription(newDatabase.chunk_preset_id)
)

const loadModels = async () => {
  modelsLoading.value = true
  try {
    const [embedding, chat] = await Promise.all([modelApi.list('embedding'), modelApi.list('chat')])
    embeddingModels.value = embedding.filter((model) => model.enabled !== false)
    chatModels.value = chat.filter((model) => model.enabled !== false)
  } catch (error) {
    console.error('加载模型列表失败:', error)
  } finally {
    modelsLoading.value = false
    sanitizePrefilledModels()
  }
}

const loadRagSettings = async () => {
  ragSettingsLoading.value = true
  try {
    await configStore.refreshRagSettings()
  } catch (error) {
    console.error('加载全局 Rerank 设置失败:', error)
  } finally {
    ragSettingsLoading.value = false
  }
}

const openRagSettings = () => {
  openSettingsModal?.('base')
}

const resetNewDatabase = () => {
  Object.assign(newDatabase, createEmptyDatabaseForm())
}

// 用系统设置里的默认模型预填创建表单（可手动修改）
const applyDefaultModelPrefill = () => {
  const config = configStore.config || {}
  newDatabase.chat_model_id = config.default_model ? Number(config.default_model) : undefined
  newDatabase.embedding_model_id = config.embed_model ? Number(config.embed_model) : undefined
  sanitizePrefilledModels()
}

// 校验预填的模型是否在可选列表中，已禁用/下线则回退为空
const sanitizePrefilledModels = () => {
  if (modelsLoading.value) return
  const chatIds = new Set(chatModelOptions.value.map((o) => Number(o.value)))
  const embedIds = new Set(embeddingModelOptions.value.map((o) => Number(o.value)))
  if (newDatabase.chat_model_id && !chatIds.has(Number(newDatabase.chat_model_id))) {
    newDatabase.chat_model_id = undefined
  }
  if (newDatabase.embedding_model_id && !embedIds.has(Number(newDatabase.embedding_model_id))) {
    newDatabase.embedding_model_id = undefined
  }
}

const openNewDatabaseModal = () => {
  resetNewDatabase()
  applyDefaultModelPrefill()
  state.openNewDatabaseModel = true
}

const cancelCreateDatabase = () => {
  state.openNewDatabaseModel = false
  resetNewDatabase()
}

// 格式化创建时间
const formatCreatedTime = (createdAt) => {
  if (!createdAt) return ''
  const parsed = parseToShanghai(createdAt)
  if (!parsed) return ''

  const today = dayjs().startOf('day')
  const createdDay = parsed.startOf('day')
  const diffInDays = today.diff(createdDay, 'day')

  if (diffInDays === 0) {
    return '今天创建'
  }
  if (diffInDays === 1) {
    return '昨天创建'
  }
  if (diffInDays < 7) {
    return `${diffInDays} 天前创建`
  }
  if (diffInDays < 30) {
    const weeks = Math.floor(diffInDays / 7)
    return `${weeks} 周前创建`
  }
  if (diffInDays < 365) {
    const months = Math.floor(diffInDays / 30)
    return `${months} 个月前创建`
  }
  const years = Math.floor(diffInDays / 365)
  return `${years} 年前创建`
}

// 构建请求数据（只负责表单数据转换）
const buildRequestData = () => buildKnowledgeBaseCreateRequest(newDatabase)

// 创建按钮处理
const handleCreateDatabase = async () => {
  if (!newDatabase.embedding_model_id) {
    message.error('请选择嵌入模型')
    return
  }
  if (!newDatabase.chat_model_id) {
    message.error('请选择问答模型')
    return
  }
  if (!newDatabase.description?.trim()) {
    message.error('请填写知识库描述')
    return
  }

  const requestData = buildRequestData()
  try {
    await databaseStore.createDatabase(requestData)
    resetNewDatabase()
    state.openNewDatabaseModel = false
  } catch {
    // 错误已在 store 中处理
  }
}

const cardSubtitle = (database) => {
  const parts = []
  if (database.created_at) {
    parts.push(formatCreatedTime(database.created_at))
  }
  parts.push(`${database.stats?.file_count || 0} 文件`)
  return parts.join(' · ')
}

const cardTags = (database) => {
  const tags = []
  const embeddingModel = embeddingModels.value.find(
    (model) => model.id === database.embedding_model_id
  )
  if (embeddingModel) {
    tags.push({
      name: embeddingModel.name,
      color: 'blue'
    })
  }
  return tags
}

const navigateToDatabase = (database) => {
  router.push({ path: `/extensions/knowledgebase/${database.kb_id}` })
}

const copyDatabaseId = async (database) => {
  try {
    await navigator.clipboard.writeText(database.kb_id)
  } catch {
    const textArea = document.createElement('textarea')
    textArea.value = database.kb_id
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
  }
  message.success('知识库 ID 已复制')
}

const deleteDatabase = (database) => {
  Modal.confirm({
    title: '删除知识库',
    content: `确定要删除知识库“${database.name}”吗？此操作不可撤销。`,
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    onOk: async () => {
      try {
        await databaseApi.deleteDatabase(database.kb_id)
        message.success('知识库已删除')
        await databaseStore.loadDatabases()
      } catch (error) {
        message.error(error.message || '删除失败')
        throw error
      }
    }
  })
}

const handleDatabaseAction = (key, database) => {
  if (key === 'copy') {
    copyDatabaseId(database)
    return
  }
  if (key === 'edit') {
    router.push({
      path: `/extensions/knowledgebase/${database.kb_id}`,
      query: { action: 'edit' }
    })
    return
  }
  if (key === 'delete') {
    deleteDatabase(database)
  }
}

watch(
  () => route.path,
  (newPath) => {
    if (newPath === '/extensions' && route.query.tab === 'knowledge') {
      databaseStore.loadDatabases()
    }
  }
)

onMounted(() => {
  loadChunkPresetOptions()
  loadModels()
  loadRagSettings()
  databaseStore.loadDatabases()
})

defineExpose({
  loading: computed(() => dbState.value.listLoading)
})
</script>

<style lang="less" scoped>
.database-container {
  :deep(.info-card-icon) {
    background: var(--gray-0);
  }
}

.new-database-modal {
  .new-database-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .form-section.compact-section {
    gap: 6px;
  }

  .settings-link {
    align-self: flex-start;
    height: auto;
    padding: 0;
  }

  .form-grid {
    display: grid;
    gap: 16px;

    &.two-columns {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    &.three-columns {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    @media (max-width: 768px) {
      &.two-columns,
      &.three-columns {
        grid-template-columns: 1fr;
      }
    }
  }

  .full-width {
    width: 100%;
  }

  .compact-model-selector {
    height: 40px;
  }

  .model-option {
    min-width: 0;
    padding: 2px 0;
    line-height: 1.35;
  }

  .model-option-title {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    color: var(--gray-800);
    font-weight: 500;
  }

  .model-option-title > span:first-child,
  .model-option-remark {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .model-option-title > span:first-child {
    min-width: 0;
  }

  .model-option-id {
    flex: 0 0 auto;
    color: var(--gray-400);
    font-size: 11px;
    font-weight: 400;
  }

  .model-option-remark {
    color: var(--gray-500);
    font-size: 12px;
  }

  .section-title {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    color: var(--gray-800);
  }

  .required-mark {
    margin-left: 2px;
    color: var(--color-error-500);
  }

  .field-hint {
    margin: 0;
    font-size: 13px;
    line-height: 1.5;
    color: var(--gray-600);
  }

  .description-hint {
    margin-top: -2px;
  }

  .chunk-preset-title-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .chunk-preset-help-icon {
    color: var(--gray-500);
    cursor: help;
    font-size: 14px;
  }

  .kb-type-guide {
    margin: 12px 0;
  }

  .privacy-config {
    display: flex;
    align-items: center;
    margin-bottom: 12px;
  }

  .kb-type-cards {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
    margin: 4px 0 0;

    @media (max-width: 768px) {
      grid-template-columns: 1fr;
      gap: 10px;
    }

    .kb-type-card {
      border: 1px solid var(--gray-150);
      border-radius: 12px;
      padding: 14px;
      cursor: pointer;
      transition: all 0.2s ease;
      background: var(--gray-0);
      position: relative;
      overflow: hidden;

      &:hover {
        border-color: var(--main-color);
      }

      &.active {
        border-color: var(--main-color);
        background: var(--main-10);
        box-shadow: 0 0 0 1px var(--main-20);

        .type-icon {
          color: var(--main-color);
        }
      }

      .card-header {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 10px;

        .type-icon {
          width: 20px;
          height: 20px;
          color: var(--main-color);
          flex-shrink: 0;
        }

        .type-title {
          font-size: 15px;
          font-weight: 600;
          color: var(--gray-800);
        }
      }

      .card-description {
        font-size: 13px;
        color: var(--gray-600);
        line-height: 1.5;
        margin-bottom: 0;
      }

      .deprecated-badge {
        background: var(--color-error-100);
        color: var(--color-error-600);
        font-size: 10px;
        font-weight: 600;
        padding: 2px 6px;
        border-radius: 4px;
        margin-left: auto;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        cursor: help;
        transition: all 0.2s ease;

        &:hover {
          background: var(--color-error-200);
          color: var(--color-error-700);
        }
      }
    }
  }

  .chunk-config {
    margin-top: 16px;
    padding: 12px 16px;
    background-color: var(--gray-25);
    border-radius: 6px;
    border: 1px solid var(--gray-150);

    h3 {
      margin-top: 0;
      margin-bottom: 12px;
      color: var(--gray-800);
    }

    .chunk-params {
      display: flex;
      flex-direction: column;
      gap: 12px;

      .param-row {
        display: flex;
        align-items: center;
        gap: 12px;

        label {
          min-width: 80px;
          font-weight: 500;
          color: var(--gray-700);
        }

        .param-hint {
          font-size: 12px;
          color: var(--gray-500);
          margin-left: 8px;
        }
      }
    }
  }
}

.database-container {
  padding: 0;
}

.loading-container {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  height: 300px;
  gap: 16px;
}

.new-database-modal {
  h3 {
    margin-top: 10px;
  }
}
</style>
