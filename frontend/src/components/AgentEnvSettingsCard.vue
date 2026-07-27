<template>
  <div class="agent-env-settings">
    <div class="header-section">
      <div class="header-content">
        <div class="section-title">沙盒环境变量</div>
        <p class="section-description">
          配置当前用户的 Agent 沙盒环境变量。新建沙盒时会注入这些变量，并覆盖同名全局 sandbox.env。
        </p>
      </div>
      <div class="header-actions">
        <a-button class="lucide-icon-btn" :loading="loading" @click="loadAgentEnv">
          <template #icon><RefreshCw :size="16" :class="{ spin: loading }" /></template>
          刷新
        </a-button>
        <a-button type="primary" :loading="saving" @click="saveAgentEnv">
          {{ saveButtonText }}
        </a-button>
      </div>
    </div>

    <div class="env-tip">保存后仅对新建沙盒生效，已运行沙盒不会热更新。</div>

    <a-spin :spinning="loading">
      <McpEnvEditor
        :key="editorRevision"
        :modelValue="draftEnv"
        :locked-keys="savedEnvKeys"
        conceal-locked-values
        @update:modelValue="updateDraftEnv"
      />
    </a-spin>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { RefreshCw } from 'lucide-vue-next'
import { agentEnvApi } from '@/apis/agent_env_api'
import McpEnvEditor from '@/components/McpEnvEditor.vue'

const ENV_KEY_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/
const MAX_ENV_COUNT = 200
const MAX_ENV_KEY_LENGTH = 128
const MAX_ENV_VALUE_LENGTH = 32768

const loading = ref(false)
const saving = ref(false)
const draftEnv = ref({})
const lastSavedEnv = ref({})
const editorRevision = ref(0)

const normalizeEnv = (env) => {
  if (!env || typeof env !== 'object' || Array.isArray(env)) {
    return {}
  }
  return Object.fromEntries(
    Object.entries(env)
      .map(([key, value]) => [key.trim(), value == null ? '' : String(value)])
      .filter(([key]) => key)
  )
}

const isSameEnv = (left, right) => {
  const leftEntries = Object.entries(left)
  const rightEntries = Object.entries(right)
  if (leftEntries.length !== rightEntries.length) return false
  return leftEntries.every(([key, value]) => right[key] === value)
}

const savedEnvKeys = computed(() => Object.keys(lastSavedEnv.value || {}))
const hasUnsavedChanges = computed(
  () => !isSameEnv(normalizeEnv(draftEnv.value), lastSavedEnv.value)
)
const saveButtonText = computed(() => (hasUnsavedChanges.value ? '保存（有修改）' : '保存'))

const updateDraftEnv = (value) => {
  const nextEnv = normalizeEnv(value)
  if (!isSameEnv(draftEnv.value, nextEnv)) {
    draftEnv.value = nextEnv
  }
}

const validateEnv = (env) => {
  const entries = Object.entries(env)
  if (entries.length > MAX_ENV_COUNT) {
    message.error(`环境变量数量不能超过 ${MAX_ENV_COUNT} 个`)
    return false
  }

  for (const [key, value] of entries) {
    if (key.length > MAX_ENV_KEY_LENGTH) {
      message.error(`环境变量名长度不能超过 ${MAX_ENV_KEY_LENGTH}`)
      return false
    }
    if (!ENV_KEY_PATTERN.test(key)) {
      message.error(`环境变量名 ${key} 格式不正确`)
      return false
    }
    if (value.length > MAX_ENV_VALUE_LENGTH) {
      message.error(`环境变量 ${key} 的值过长`)
      return false
    }
  }
  return true
}

const loadAgentEnv = async () => {
  loading.value = true
  try {
    const res = await agentEnvApi.get()
    const env = normalizeEnv(res.env)
    draftEnv.value = env
    lastSavedEnv.value = env
    editorRevision.value += 1
  } catch (error) {
    message.error(error.message || '加载环境变量失败')
  } finally {
    loading.value = false
  }
}

const saveAgentEnv = async () => {
  const env = normalizeEnv(draftEnv.value)
  if (!validateEnv(env)) return
  if (isSameEnv(env, lastSavedEnv.value)) {
    message.info('环境变量未变化')
    return
  }

  saving.value = true
  try {
    await agentEnvApi.update(env)
    draftEnv.value = env
    lastSavedEnv.value = env
    editorRevision.value += 1
    message.success('环境变量已保存')
  } catch (error) {
    message.error(error.message || '保存环境变量失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadAgentEnv)
</script>

<style lang="less" scoped>
.agent-env-settings {
  .header-section {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    gap: 16px;
    margin-bottom: 12px;

    @media (max-width: 760px) {
      align-items: stretch;
      flex-direction: column;
    }
  }

  .header-content {
    flex: 1;
    min-width: 0;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }

  .env-tip {
    margin-bottom: 14px;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--main-10);
    border: 1px solid var(--main-300);
    color: var(--main-700);
    font-size: 13px;
    line-height: 1.5;
  }
}

:deep(.spin) {
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
</style>
