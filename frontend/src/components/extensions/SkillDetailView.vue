<template>
  <div class="skill-detail extension-detail-page">
    <div v-if="loading" class="loading-bar-wrapper">
      <div class="loading-bar"></div>
    </div>
    <div class="detail-top-bar">
      <button class="detail-back-btn" @click="goBack">
        <ArrowLeft :size="16" />
        <span>返回</span>
      </button>
      <div class="detail-title-area">
        <div class="detail-icon">
          <WandSparkles :size="18" />
        </div>
        <div class="detail-title-text">
          <h2>{{ currentSkill?.name || slug }}</h2>
          <span class="detail-subtitle">{{ currentSkillStatusLabel }}</span>
        </div>
      </div>
      <div class="detail-actions">
        <a-space :size="8">
          <button
            v-if="isInstalledSkill && canManageCurrentSkill"
            type="button"
            @click="handleExport"
            class="lucide-icon-btn extension-panel-action extension-panel-action-secondary"
          >
            <Download :size="14" />
            <span>导出</span>
          </button>
          <button
            v-if="isInstalledSkill && canManageCurrentSkill && !isBuiltinInstalledSkill"
            type="button"
            @click="confirmDeleteSkill"
            class="lucide-icon-btn extension-panel-action extension-panel-action-danger"
          >
            <Trash2 :size="14" />
            <span>删除</span>
          </button>
        </a-space>
      </div>
    </div>

    <div class="detail-content-wrapper">
      <div v-if="currentSkill" class="detail-content-inner">
        <div v-if="isReadOnlySkill" class="readonly-scope-hint readonly-detail-hint">
          你可以查看并使用此 Skill，但没有管理权限。
        </div>
        <a-tabs v-if="isInstalledSkill" v-model:activeKey="activeTab" class="minimal-tabs">
          <a-tab-pane key="editor">
            <template #tab>
              <span class="tab-title"><FileText :size="14" />代码管理</span>
            </template>
            <div class="workspace">
              <div class="tree-container">
                <div class="tree-header">
                  <span class="label">项目结构</span>
                  <div class="tree-actions">
                    <a-tooltip title="刷新"
                      ><button @click="reloadTree"><RotateCw :size="14" /></button
                    ></a-tooltip>
                  </div>
                </div>
                <div class="tree-content">
                  <FileTreeComponent
                    v-model:selectedKeys="selectedTreeKeys"
                    v-model:expandedKeys="expandedKeys"
                    :tree-data="treeData"
                    @select="handleTreeSelect"
                  />
                </div>
              </div>
              <div class="editor-container">
                <div class="editor-main">
                  <a-empty
                    v-if="!selectedPath || selectedIsDir"
                    description="选择文件以开始编辑"
                    class="mt-40"
                  />
                  <template v-else>
                    <AgentFilePreview
                      :file="selectedFilePreview"
                      :file-path="selectedPath"
                      :show-download="false"
                      :show-fullscreen="true"
                      :editable="canEditSkillFiles"
                      :edit-all-text="true"
                      :saving="savingFile"
                      :full-height="true"
                      container-class="skill-file-preview"
                      content-class="skill-file-preview-content"
                      @save="saveCurrentFile"
                    />
                  </template>
                </div>
              </div>
            </div>
          </a-tab-pane>

        </a-tabs>
      </div>
      <div v-else-if="!loading" class="detail-empty">
        <a-empty description="未找到 Skill" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message, Modal } from 'ant-design-vue'
import { ArrowLeft, WandSparkles, Download, Trash2, FileText, RotateCw } from 'lucide-vue-next'
import { skillApi } from '@/apis/skill_api'
import AgentFilePreview from '@/components/AgentFilePreview.vue'
import FileTreeComponent from '@/components/FileTreeComponent.vue'

const route = useRoute()
const router = useRouter()
const slug = computed(() => decodeURIComponent(route.params.slug))

const loading = ref(false)
const currentSkill = ref(null)
const treeData = ref([])
const selectedTreeKeys = ref([])
const expandedKeys = ref([])
const selectedPath = ref('')
const selectedIsDir = ref(false)
const fileContent = ref('')
const savingFile = ref(false)
const activeTab = ref('editor')

const skills = ref([])

const isInstalledSkill = computed(() => !!currentSkill.value?.dir_path)

const isBuiltinInstalledSkill = computed(() => {
  return !!(isInstalledSkill.value && currentSkill.value?.source_type === 'builtin')
})
const canManageCurrentSkill = computed(() => currentSkill.value?.can_manage !== false)
const isReadOnlySkill = computed(() => isInstalledSkill.value && !canManageCurrentSkill.value)
const canEditSkillFiles = computed(
  () => canManageCurrentSkill.value && !isBuiltinInstalledSkill.value
)

const sourceTypeLabel = (sourceType) => {
  if (sourceType === 'builtin') return '内置'
  if (sourceType === 'remote') return '远程添加'
  return '上传'
}

const currentSkillStatusLabel = computed(() => {
  const skill = currentSkill.value
  if (!skill) return ''
  if (skill.enabled === false) return `${sourceTypeLabel(skill.source_type)} · 已禁用`
  return sourceTypeLabel(skill.source_type)
})

const selectedFilePreview = computed(() => ({
  content: fileContent.value,
  previewType: 'text',
  supported: true
}))

const goBack = () => {
  router.push({ path: '/tools', query: { tab: 'skills' } })
}

const fetchSkillDetail = async () => {
  loading.value = true
  try {
    const skillResult = await skillApi.listSkills()
    skills.value = skillResult?.data || []
    const found = skills.value.find((s) => s.slug === slug.value)
    if (found) {
      currentSkill.value = found
      await reloadTree()
      await loadSkillFile(found.slug)
    }
  } catch {
    message.error('加载失败')
  } finally {
    loading.value = false
  }
}

const normalizeTree = (node) => {
  if (!node) return []
  // 如果是数组，递归处理每个元素
  if (Array.isArray(node)) {
    return node.map(normalizeTree).flat()
  }
  // 单个节点对象
  return [{
    title: node.name,
    key: node.path,
    isLeaf: !node.is_dir,
    path: node.path,
    is_dir: node.is_dir,
    children: node.is_dir ? normalizeTree(node.children || []) : undefined
  }]
}

const resetFileState = () => {
  selectedPath.value = ''
  selectedIsDir.value = false
  selectedTreeKeys.value = []
  expandedKeys.value = []
  fileContent.value = ''
}

const expandAllKeys = (nodes) =>
  nodes.flatMap((node) => (node.is_dir ? [node.key, ...expandAllKeys(node.children || [])] : []))

const reloadTree = async () => {
  if (!currentSkill.value || !isInstalledSkill.value) return
  loading.value = true
  try {
    const result = await skillApi.getSkillTree(currentSkill.value.slug)
    const normalized = normalizeTree(result?.data || [])
    treeData.value = normalized
    expandedKeys.value = expandAllKeys(normalized)
  } catch {
    message.error('加载目录树失败')
  } finally {
    loading.value = false
  }
}

const loadSkillFile = async (skillSlug, path = 'SKILL.md') => {
  try {
    const fileResult = await skillApi.getSkillFile(skillSlug, path)
    const content = fileResult?.data?.content || ''
    fileContent.value = content
    selectedPath.value = path
    selectedIsDir.value = false
    selectedTreeKeys.value = [path]
  } catch {
    // file not found is ok
  }
}

const handleTreeSelect = async (keys, info) => {
  if (!keys?.length) {
    resetFileState()
    return
  }
  const node = info?.node || {}
  const path = node.path || node.key
  const isDir = !!node.is_dir
  selectedTreeKeys.value = [path]
  selectedPath.value = path
  selectedIsDir.value = isDir
  if (isDir) {
    fileContent.value = ''
    return
  }
  try {
    const result = await skillApi.getSkillFile(currentSkill.value.slug, path)
    const content = result?.data?.content || ''
    fileContent.value = content
  } catch {
    message.error('文件读取失败')
  }
}

const saveCurrentFile = async (content = fileContent.value) => {
  if (!currentSkill.value || !selectedPath.value || selectedIsDir.value || !canEditSkillFiles.value)
    return
  savingFile.value = true
  try {
    await skillApi.updateSkillFile(currentSkill.value.slug, {
      path: selectedPath.value,
      content
    })
    fileContent.value = content
    message.success('已保存')
    if (selectedPath.value === 'SKILL.md') await fetchSkillDetail()
  } catch {
    message.error('保存失败')
  } finally {
    savingFile.value = false
  }
}

const confirmDeleteSkill = () => {
  const target = currentSkill.value
  if (!target || !canManageCurrentSkill.value || isBuiltinInstalledSkill.value) return
  const actionText = '删除'
  Modal.confirm({
    title: `确认${actionText}技能「${target.slug}」？`,
    content: '删除后无法恢复，所有文件和配置将永久消失。',
    okText: `确认${actionText}`,
    okType: 'danger',
    cancelText: '取消',
    onOk: async () => {
      try {
        await skillApi.deleteSkill(target.slug)
        message.success(`已${actionText}`)
        router.push({ path: '/tools', query: { tab: 'skills' } })
      } catch {
        message.error(`${actionText}失败`)
      }
    }
  })
}

const handleExport = async () => {
  if (!currentSkill.value || !isInstalledSkill.value || !canManageCurrentSkill.value) return
  try {
    const response = await skillApi.exportSkill(currentSkill.value.slug)
    const blob = await response.blob()
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${currentSkill.value.slug}.zip`
    link.click()
    URL.revokeObjectURL(url)
  } catch {
    message.error('导出失败')
  }
}

onMounted(() => {
  fetchSkillDetail()
})
</script>

<style lang="less" scoped>
@import '@/assets/css/extensions.less';
@import '@/assets/css/extension-detail.less';

.skill-detail {
  .detail-content-wrapper {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    background-color: var(--gray-10);
  }

  .detail-content-inner {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  :deep(.minimal-tabs) {
    height: 100%;
  }
}

.workspace {
  display: flex;
  flex: 1;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.tree-container {
  width: 240px;
  order: 2;
  border-left: 1px solid var(--gray-150);
  background: var(--gray-0);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;

  .tree-header {
    padding: 10px var(--page-padding) 0;
    display: flex;
    justify-content: space-between;
    align-items: center;
    .label {
      font-size: 12px;
      font-weight: 600;
      color: var(--gray-500);
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }
    .tree-actions {
      display: flex;
      gap: 4px;
      button {
        background: none;
        border: none;
        padding: 2px;
        cursor: pointer;
        color: var(--gray-500);
        display: flex;
        align-items: center;
        &:hover {
          color: var(--gray-900);
        }
      }
    }
  }

  .tree-content {
    flex: 1;
    overflow-y: auto;
    height: 100%;
    padding: 8px calc(var(--page-padding) - 4px);
  }
}

.editor-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;

  .editor-main {
    flex: 1;
    min-height: 0;
    background-color: var(--gray-0);
    display: flex;
    flex-direction: column;
  }

  .editor-main :deep(.ant-empty) {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .skill-file-preview {
    flex: 1;
    min-height: 0;
    border-radius: 0;
  }

  :deep(.skill-file-preview-content) {
    flex: 1;
    min-height: 0;
    max-height: none;
  }

  :deep(.skill-file-preview-content .file-content-pre.code-highlight code) {
    min-height: 100%;
  }
}

.settings-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.settings-card {
  border: 1px solid var(--gray-150);
  border-radius: 10px;
  background: var(--gray-0);
}

.settings-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 12px;

  &.scope-card {
    display: block;
  }
}

.settings-card-main {
  min-width: 0;
}

.settings-card-title {
  margin-bottom: 4px;
  color: var(--gray-900);
  font-size: 14px;
  font-weight: 700;
}

.settings-card-desc {
  color: var(--gray-500);
  font-size: 13px;
  line-height: 1.55;
}

.settings-card-action {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 10px;
}

.scope-card .settings-card-main {
  margin-bottom: 14px;
}

.status-pill {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  line-height: 18px;

  &.enabled {
    background: var(--main-10);
    color: var(--main-color);
  }

  &.disabled {
    background: var(--gray-100);
    color: var(--gray-500);
  }
}

.readonly-scope-hint {
  color: var(--gray-500);
  background: var(--gray-50);
  border: 1px solid var(--gray-150);
  border-radius: 10px;
  padding: 11px 12px;
  font-size: 13px;
  line-height: 1.55;
}

@media (max-width: 768px) {
  .settings-card {
    flex-direction: column;
    align-items: stretch;
  }
}

.mt-40 {
  margin-top: 40px;
}
.pt-12 {
  padding-top: 12px;
}
</style>
