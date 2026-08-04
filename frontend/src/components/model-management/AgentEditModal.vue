<script setup>
import { computed, nextTick, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import {
  Info,
  RefreshCw,
  Settings2,
  SlidersHorizontal,
  Upload,
  Wrench
} from 'lucide-vue-next'

import { userApi } from '@/apis/user_api'
import AgentRuntimeConfigForm from '@/components/AgentRuntimeConfigForm.vue'
import FallbackAvatar from '@/components/common/FallbackAvatar.vue'
import { useAgentStore } from '@/stores/agent'
import { generatePixelAvatar } from '@/utils/pixelAvatar'
import { MAX_IMAGE_UPLOAD_SIZE_BYTES, MAX_IMAGE_UPLOAD_SIZE_MB } from '@/utils/upload_limits'

const props = defineProps({
  backendOptions: { type: Array, default: () => [] }
})

const emit = defineEmits(['saved'])

const agentStore = useAgentStore()

const DEFAULT_AGENT_BACKEND_ID = 'ChatbotAgent'
const SUB_AGENT_BACKEND_ID = 'SubAgentBackend'
const runtimeAgentModalTabs = ['model', 'tools', 'other']

const showAgentModal = ref(false)
const editingAgentId = ref(null)
const agentModalActiveTab = ref('basic')
const agentIconUploading = ref(false)
const saving = ref(false)
const runtimeConfigFormRef = ref(null)
const agentNameInputRef = ref(null)
const agentForm = reactive({
  slug: '',
  name: '',
  backend_id: DEFAULT_AGENT_BACKEND_ID,
  description: '',
  icon: ''
})

const normalizeAgent = (agent) => {
  const agentId = agent?.agent_id || agent?.slug || agent?.id
  return agentId
    ? { ...agent, id: agentId, agent_id: agentId, slug: agent?.slug || agentId }
    : agent
}

const agentModalMenuItems = computed(() => {
  const items = [{ key: 'basic', label: '基本信息', icon: Info }]
  if (editingAgentId.value) {
    items.push(
      { key: 'model', label: '模型配置', icon: SlidersHorizontal },
      { key: 'tools', label: '工具配置', icon: Wrench },
      { key: 'other', label: '其他配置', icon: Settings2 }
    )
  }
  return items
})

const showAgentModalSidebar = computed(() => agentModalMenuItems.value.length > 1)
const runtimeConfigSegment = computed(() =>
  runtimeAgentModalTabs.includes(agentModalActiveTab.value) ? agentModalActiveTab.value : 'model'
)
const isRuntimeAgentModalTab = (key) => runtimeAgentModalTabs.includes(key)
const getDefaultBackendId = () => DEFAULT_AGENT_BACKEND_ID
const isSubAgentBackend = (backendId) => backendId === SUB_AGENT_BACKEND_ID

const agentModalTitle = computed(() => (editingAgentId.value ? '编辑智能体' : '新增智能体'))
const agentPreviewDefaultIcon = computed(() =>
  editingAgentId.value ? generatePixelAvatar(editingAgentId.value) : ''
)
const agentPreviewName = computed(() => agentForm.name || editingAgentId.value || '智能体')
const selectedBackendOption = computed(() =>
  props.backendOptions.find((backend) => backend.value === agentForm.backend_id)
)
const selectedBackendLabel = computed(
  () => selectedBackendOption.value?.label || agentForm.backend_id || '未选择'
)

const resetAgentForm = () => {
  Object.assign(agentForm, {
    slug: '',
    name: '',
    backend_id: getDefaultBackendId(),
    description: '',
    icon: ''
  })
}

const focusAgentNameInput = async () => {
  await nextTick()
  agentNameInputRef.value?.focus?.()
}

const handleAgentModalAfterOpenChange = (open) => {
  if (open && !editingAgentId.value) focusAgentNameInput()
}

const openCreate = () => {
  editingAgentId.value = null
  agentModalActiveTab.value = 'basic'
  resetAgentForm()
  agentStore.resetAgentConfig()
  showAgentModal.value = true
}

const openEdit = async (agent) => {
  const agentId = typeof agent === 'string' ? agent : agent?.id
  if (!agentId) return

  const detail = await agentStore.fetchAgentDetail(agentId, true)
  if (!detail?.can_manage) {
    message.warning('当前智能体不可编辑')
    return
  }

  editingAgentId.value = detail.id
  agentModalActiveTab.value = 'basic'
  Object.assign(agentForm, {
    slug: detail.id || detail.slug || '',
    name: detail.name || '',
    backend_id: detail.backend_id || DEFAULT_AGENT_BACKEND_ID,
    description: detail.description || '',
    icon: detail.icon || ''
  })
  await agentStore.selectAgent(detail.id, { allowSubagent: true })
  showAgentModal.value = true
}

const restoreChatAgentSelectionIfNeeded = async () => {
  if (!agentStore.selectedAgent?.is_subagent) return
  const fallbackAgentId = (agentStore.agents || []).find((agent) => !agent.is_subagent)?.id
  if (fallbackAgentId) await agentStore.selectAgent(fallbackAgentId)
}

const closeAgentModal = async () => {
  if (saving.value || agentIconUploading.value) return
  showAgentModal.value = false
  await restoreChatAgentSelectionIfNeeded()
}

const beforeAgentIconUpload = (file) => {
  if (!file.type.startsWith('image/')) {
    message.error('只能上传图片文件')
    return false
  }

  if (file.size > MAX_IMAGE_UPLOAD_SIZE_BYTES) {
    message.error(`图片大小不能超过 ${MAX_IMAGE_UPLOAD_SIZE_MB}MB`)
    return false
  }

  uploadAgentIcon(file)
  return false
}

const uploadAgentIcon = async (file) => {
  agentIconUploading.value = true
  try {
    const data = await userApi.uploadImage(file)
    agentForm.icon = data.image_url || data.url || ''
    message.success('图标上传成功')
  } catch (error) {
    message.error(error.message || '图标上传失败')
  } finally {
    agentIconUploading.value = false
  }
}

const buildAgentPayload = () => {
  const payload = {
    name: agentForm.name.trim(),
    description: agentForm.description.trim() || null,
    icon: agentForm.icon.trim() || null,
    is_subagent: isSubAgentBackend(agentForm.backend_id)
  }

  if (!editingAgentId.value) {
    payload.backend_id = agentForm.backend_id
  }

  return payload
}

const saveAgent = async () => {
  if (!agentForm.name.trim()) {
    agentModalActiveTab.value = 'basic'
    message.error('请填写智能体名称')
    return
  }

  saving.value = true
  try {
    const payload = buildAgentPayload()
    if (editingAgentId.value) {
      const updated = await agentStore.updateAgentProfile(editingAgentId.value, payload)
      agentStore.originalAgentConfig = { ...agentStore.agentConfig }
      emit('saved', { mode: 'edit', agent: updated })
      message.success('智能体已保存')
    } else {
      const created = await agentStore.createAgent(payload)
      emit('saved', { mode: 'create', agent: normalizeAgent(created) })
      message.success('智能体已创建')
    }
    showAgentModal.value = false
    await restoreChatAgentSelectionIfNeeded()
  } catch (error) {
    message.error(error.message || '保存智能体失败')
  } finally {
    saving.value = false
  }
}

defineExpose({
  openCreate,
  openEdit,
  close: closeAgentModal
})
</script>

<template>
  <a-modal
    v-model:open="showAgentModal"
    class="agent-edit-modal"
    :width="editingAgentId ? 820 : 740"
    :footer="null"
    :closable="false"
    @cancel="closeAgentModal"
    @after-open-change="handleAgentModalAfterOpenChange"
  >
    <template #title>
      <div class="agent-modal-titlebar">
        <span class="agent-modal-title">{{ agentModalTitle }}</span>
        <div class="agent-modal-actions">
          <a-button :disabled="saving" @click="closeAgentModal">取消</a-button>
          <a-button type="primary" :loading="saving" @click="saveAgent">
            {{ agentStore.hasConfigChanges ? '保存（有修改）' : '保存' }}
          </a-button>
        </div>
      </div>
    </template>
    <div
      class="agent-modal-content"
      :class="{
        'without-sidebar': !showAgentModalSidebar,
        'create-mode': !editingAgentId
      }"
    >
      <aside v-if="showAgentModalSidebar" class="agent-modal-sidebar" aria-label="智能体配置分组">
        <button
          v-for="item in agentModalMenuItems"
          :key="item.key"
          type="button"
          class="agent-modal-nav-item"
          :class="{ active: agentModalActiveTab === item.key }"
          @click="agentModalActiveTab = item.key"
        >
          <span class="nav-item-main">
            <component :is="item.icon" :size="16" />
            <span>{{ item.label }}</span>
          </span>
          <span v-if="item.key === 'model' && agentStore.hasConfigChanges" class="nav-dirty-dot" />
        </button>
      </aside>

      <div class="agent-modal-main">
        <section v-show="agentModalActiveTab === 'basic'" class="agent-modal-section">
          <div class="agent-profile-header">
            <div class="agent-icon-preview" aria-label="智能体图标与名称">
              <div class="agent-profile-main">
                <a-upload
                  :show-upload-list="false"
                  :before-upload="beforeAgentIconUpload"
                  :disabled="agentIconUploading"
                  accept="image/*"
                >
                  <div
                    class="agent-icon-upload"
                    :class="{
                      uploading: agentIconUploading,
                      'is-empty': !agentForm.icon && !editingAgentId
                    }"
                  >
                    <FallbackAvatar
                      v-if="agentForm.icon || editingAgentId"
                      :src="agentForm.icon"
                      :default-src="agentPreviewDefaultIcon"
                      :name="agentPreviewName"
                      :seed="editingAgentId || agentForm.slug || agentForm.name"
                      kind="agent"
                      :size="56"
                      shape="rounded"
                      :alt="`${agentForm.name || '智能体'}图标`"
                      class="agent-icon-preview-avatar"
                    />
                    <div class="agent-icon-mask">
                      <RefreshCw v-if="agentIconUploading" :size="16" class="spinning" />
                      <Upload v-else :size="16" />
                      <span>{{ agentForm.icon ? '更换图标' : '上传图标' }}</span>
                    </div>
                  </div>
                </a-upload>
                <div class="agent-icon-preview-text">
                  <div class="agent-name-field">
                    <span class="required-mark" aria-hidden="true">*</span>
                    <input
                      ref="agentNameInputRef"
                      v-model="agentForm.name"
                      class="agent-inline-name-input"
                      type="text"
                      placeholder="请输入智能体名称"
                      aria-label="智能体名称"
                    />
                  </div>
                  <span v-if="editingAgentId" class="agent-inline-slug">{{
                    agentForm.slug || editingAgentId
                  }}</span>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-form">
            <label class="form-label full-width">
              <span>智能体后端</span>
              <a-select
                v-if="!editingAgentId"
                v-model:value="agentForm.backend_id"
                class="agent-backend-select"
                :options="backendOptions"
              />
              <span v-else class="agent-backend-name">{{ selectedBackendLabel }}</span>
            </label>
            <label class="form-label full-width">
              <span>描述</span>
              <a-textarea
                v-model:value="agentForm.description"
                class="agent-description-textarea"
                :rows="3"
                placeholder="可选"
              />
            </label>
          </div>
        </section>

        <section
          v-if="editingAgentId"
          v-show="isRuntimeAgentModalTab(agentModalActiveTab)"
          class="agent-modal-section runtime-section"
        >
          <AgentRuntimeConfigForm
            ref="runtimeConfigFormRef"
            :segment="runtimeConfigSegment"
            :show-segmented="false"
          />
        </section>
      </div>
    </div>
  </a-modal>
</template>

<style lang="less" scoped>
.agent-modal-titlebar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
}

.agent-modal-title {
  color: var(--gray-900);
  font-size: 16px;
  font-weight: 600;
}

.agent-modal-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;

  :deep(.ant-btn) {
    min-width: 70px;
    height: 36px;
    border-radius: 8px;
    font-weight: 500;
  }

  :deep(.ant-btn-primary) {
    border-color: var(--main-700);
    background: var(--main-700);

    &:hover,
    &:focus {
      border-color: var(--main-800);
      background: var(--main-800);
    }
  }
}

.agent-modal-content {
  display: grid;
  grid-template-columns: 144px minmax(0, 1fr);
  height: min(72vh, 640px);
  min-height: 0;
  overflow: hidden;
  background: var(--gray-0);

  &.without-sidebar {
    grid-template-columns: minmax(0, 1fr);
  }

  &.create-mode {
    height: auto;
    min-height: 360px;
  }
}

.agent-modal-sidebar {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-height: 0;
  padding: 14px 10px;
  overflow-y: auto;
  border-right: 1px solid var(--gray-150);
  background: transparent;
}

.agent-modal-nav-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-height: 38px;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--gray-800);
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
  transition:
    background 0.16s ease,
    border-color 0.16s ease,
    color 0.16s ease;

  &:hover {
    background: var(--gray-50);
    color: var(--gray-900);
  }

  &:focus-visible {
    outline: 2px solid var(--main-100);
    outline-offset: 1px;
    border-color: var(--main-200);
  }

  &.active {
    background: var(--main-30);
    color: var(--main-800);

    span {
      font-weight: 600;
    }
  }
}

.nav-item-main {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  gap: 8px;

  svg {
    flex-shrink: 0;
    color: var(--gray-600);
  }
}

.agent-modal-nav-item.active .nav-item-main svg {
  color: var(--main-700);
}

.nav-dirty-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-warning-600);
}

.agent-modal-main {
  min-width: 0;
  min-height: 0;
  overflow: hidden auto;
  overscroll-behavior: contain;
  padding: 22px 18px 24px 24px;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
  scrollbar-color: var(--gray-300) transparent;

  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    border: 2px solid transparent;
    border-radius: 999px;
    background: var(--gray-300);
    background-clip: content-box;
  }

  &::-webkit-scrollbar-thumb:hover {
    background: var(--gray-400);
    background-clip: content-box;
  }
}

.agent-modal-section {
  min-height: 0;
  background: var(--gray-0);
}

.runtime-section {
  display: flex;
  flex-direction: column;
  min-height: 100%;

  :deep(.agent-runtime-config-form) {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
    background: transparent;
  }

  :deep(.runtime-config-content) {
    flex: 1;
    min-width: 0;
    min-height: 0;
    padding: 0;
    overflow: visible;
  }
}

.agent-profile-header {
  margin-bottom: 20px;
}

.agent-icon-preview {
  display: flex;
  align-items: center;
  width: 100%;
  min-width: 0;
  gap: 16px;

  :deep(.ant-upload) {
    display: block;
  }
}

.agent-profile-main {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  gap: 10px;
}

.agent-icon-upload {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  overflow: hidden;
  border: 1px solid var(--gray-200);
  border-radius: 12px;
  background: var(--main-30);
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    box-shadow 0.16s ease;

  .agent-icon-preview-avatar {
    width: 100%;
    height: 100%;
    border: 0;
  }

  &:hover,
  &:focus-within,
  &.uploading {
    border-color: var(--main-300);
    box-shadow: 0 0 0 3px var(--main-50);
  }

  &:hover .agent-icon-mask,
  &:focus-within .agent-icon-mask,
  &.uploading .agent-icon-mask,
  &.is-empty .agent-icon-mask {
    opacity: 1;
  }

  &.is-empty {
    border-style: dashed;
    background: var(--gray-0);
  }
}

.agent-icon-mask {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  background: color-mix(in srgb, var(--gray-900) 62%, transparent);
  color: var(--gray-0);
  font-size: 11px;
  font-weight: 600;
  opacity: 0;
  transition: opacity 0.16s ease;
}

.agent-icon-upload.is-empty .agent-icon-mask {
  background: transparent;
  color: var(--gray-600);
}

.agent-icon-preview-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 4px;
  line-height: 1.25;
}

.agent-name-field {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.required-mark {
  flex-shrink: 0;
  color: var(--color-error-500);
  font-size: 14px;
  font-weight: 600;
  line-height: 1;
}

.agent-inline-name-input {
  width: 220px;
  max-width: 100%;
  padding: 4px 6px;
  border: 1px solid var(--gray-200);
  border-radius: 6px;
  background: var(--gray-10);
  color: var(--gray-900);
  caret-color: var(--main-700);
  font-size: 14px;
  font-weight: 600;
  line-height: 1.35;
  transition:
    border-color 0.16s ease,
    background 0.16s ease,
    box-shadow 0.16s ease;

  &::placeholder {
    color: var(--gray-400);
  }

  &:hover {
    border-color: var(--gray-300);
    background: var(--gray-0);
  }

  &:focus {
    border-color: var(--main-300);
    background: var(--gray-0);
    box-shadow: 0 0 0 3px var(--main-50);
    outline: none;
  }
}

.agent-inline-slug {
  padding: 1px 4px;
  width: 200px;
  max-width: 100%;
  overflow: hidden;
  color: var(--gray-500);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-backend-name {
  max-width: 320px;
  overflow: hidden;
  color: var(--gray-900);
  font-size: 13px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-backend-select {
  width: 280px;
  max-width: 100%;

  :deep(.ant-select-selector) {
    border-radius: 8px;
  }

  :deep(.ant-select-selection-item) {
    color: var(--gray-900);
    font-size: 13px;
    font-weight: 500;
  }
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-label {
  display: flex;
  flex-direction: column;
  gap: 6px;

  > span {
    color: var(--gray-700);
    font-size: 12px;
    font-weight: 500;
  }
}

.agent-description-textarea {
  min-height: 80px;
  padding: 10px 12px;
  border-color: var(--gray-200);
  border-radius: 8px;
  background: var(--gray-10);
  color: var(--gray-900);
  font-size: 13px;
  line-height: 1.6;
  resize: vertical;
  transition:
    border-color 0.16s ease,
    background 0.16s ease,
    box-shadow 0.16s ease;

  &::placeholder {
    color: var(--gray-400);
  }

  &:hover {
    border-color: var(--gray-300);
    background: var(--gray-0);
  }

  &:focus {
    border-color: var(--main-300);
    background: var(--gray-0);
    box-shadow: 0 0 0 3px var(--main-50);
  }
}

.full-width {
  grid-column: 1 / -1;
}

.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 768px) {
  .agent-modal-content {
    grid-template-columns: 1fr;
    height: min(78vh, 680px);
  }

  .agent-modal-sidebar {
    flex-direction: row;
    overflow-x: auto;
    border-right: 0;
    border-bottom: 1px solid var(--gray-150);
  }
}

:global(.agent-edit-modal .ant-modal-content) {
  overflow: hidden;
  padding: 0;
  border-radius: 12px;
}

:global(.agent-edit-modal .ant-modal-header) {
  margin: 0;
  padding: 18px 24px;
  border-bottom: 1px solid var(--gray-150);
  background: var(--gray-0);
}

:global(.agent-edit-modal .ant-modal-title) {
  width: 100%;
}

:global(.agent-edit-modal .ant-modal-body) {
  padding: 0;
}
</style>
