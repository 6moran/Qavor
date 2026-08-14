<template>
  <div class="ocr-settings-section">
    <div class="section-title">默认 OCR 方法</div>
    <div class="settings-panel">
      <div class="card card-select">
        <span class="label">{{ items?.default_ocr_engine?.des || '默认 OCR 方法' }}</span>
        <div class="setting-control">
          <OCRSelector
            :model-value="configStore.config?.default_ocr_engine"
            @update:model-value="configStore.setConfigValue('default_ocr_engine', $event)"
          />
        </div>
      </div>
    </div>

    <div class="section-title">OCR 服务配置</div>
    <p class="section-description">
      仅展示需要配置的服务。保存空值会清除数据库配置，并读取对应环境变量。
    </p>
    <div class="settings-panel">
      <div
        v-for="option in configOptions"
        :key="option.key"
        class="card card-select ocr-option"
      >
        <div class="option-meta">
          <span class="label">{{ option.name }}</span>
          <span class="option-desc">{{ option.description }}</span>
        </div>
        <div class="setting-control">
          <div class="option-fields">
            <label v-for="field in option.params.fields" :key="field.key" class="option-field">
              <span class="setting-label">{{ field.label }}</span>
              <a-input-password
                v-if="field.sensitive"
                v-model:value="drafts[option.key][field.key]"
                :placeholder="field.environment"
                autocomplete="new-password"
              />
              <a-input
                v-else
                v-model:value="drafts[option.key][field.key]"
                :placeholder="field.placeholder || field.environment"
                allow-clear
              />
              <small v-if="!field.sensitive">{{ field.help }}</small>
              <small v-else>留空并保存会清除数据库中的值。</small>
            </label>
          </div>
          <a-button
            type="primary"
            size="small"
            class="save-button"
            :loading="savingOption === option.key"
            @click="saveOption(option)"
          >
            保存
          </a-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { configOptionsApi } from '@/apis/system_api'
import { useConfigStore } from '@/stores/config'
import OCRSelector from '@/components/OCRSelector.vue'

const configStore = useConfigStore()
// 仅保留通用 OCR API 一项；其余自托管/官方引擎当前未接入，刻意不在界面暴露。
const OCR_OPTION_KEYS = new Set(['ocr_api_opts'])

// 通用 OCR API 的内置配置定义：作为兜底入口，即使后端 options 接口暂不可用
// （后端未重启/接口未实现），也保证「通用 OCR API」配置卡片可见可编辑。
const FALLBACK_OCR_API_OPTION = {
  key: 'ocr_api_opts',
  name: '通用 OCR API',
  description:
    '通过 HTTP 接口调用外部 OCR 服务（如硅基流动等平台）。PDF 由系统逐页渲染后上传识别，图片直接上传。',
  value: {},
  sensitive_state: { api_key: { source: 'none' } },
  params: {
    fields: [
      {
        key: 'base_url',
        label: '服务地址',
        sensitive: false,
        environment: 'QAVOR_OCR_API_BASE_URL',
        placeholder: 'https://api.example.com/v1/ocr',
        help: '接收图片上传的 OCR 接口地址，图片以 multipart 文件字段 image 提交。'
      },
      {
        key: 'api_key',
        label: 'API Key',
        sensitive: true,
        environment: 'QAVOR_OCR_API_KEY',
        help: '接口访问凭证，随请求头 Authorization: Bearer <key> 发送。'
      },
      {
        key: 'model',
        label: '模型名称',
        sensitive: false,
        environment: 'QAVOR_OCR_MODEL',
        placeholder: 'Qwen2.5-VL-72B-Instruct',
        help: '使用哪个 OCR/视觉模型，随请求以表单字段 model 提交；部分服务可不填。'
      }
    ]
  }
}

const normalizeOption = (option) => ({
  ...option,
  value: { ...(option.value || {}) },
  sensitive_state: { ...(option.sensitive_state || {}) }
})

const cloneFallbackOption = () => ({
  ...FALLBACK_OCR_API_OPTION,
  value: { ...FALLBACK_OCR_API_OPTION.value },
  sensitive_state: { ...FALLBACK_OCR_API_OPTION.sensitive_state },
  params: {
    ...FALLBACK_OCR_API_OPTION.params,
    fields: FALLBACK_OCR_API_OPTION.params.fields.map((field) => ({ ...field }))
  }
})

const items = computed(() => configStore.config?._config_items || {})
const configOptions = ref([])
const drafts = reactive({})
const savingOption = ref('')

const loadConfigOptions = async () => {
  try {
    const data = await configOptionsApi.getOptions()
    configOptions.value = (data.options || [])
      .filter((option) => OCR_OPTION_KEYS.has(option.key))
      .map((option) => {
        const normalized = normalizeOption(option)
        drafts[option.key] = { ...(normalized.value || {}) }
        return normalized
      })
  } catch (error) {
    // 接口不可用时降级为内置入口，保证配置入口始终可见
    console.warn('加载 OCR 服务配置失败，展示内置配置入口:', error)
    message.warning('配置服务接口暂不可用，已展示内置配置入口')
  }
  if (!configOptions.value.some((option) => option.key === 'ocr_api_opts')) {
    const fb = cloneFallbackOption()
    drafts[fb.key] = { ...(fb.value || {}) }
    configOptions.value.unshift(fb)
  }
}

const saveOption = async (option) => {
  savingOption.value = option.key
  try {
    const data = await configOptionsApi.updateOption(option.key, drafts[option.key] || {})
    Object.assign(option, data.option, {
      value: { ...(data.option.value || {}) },
      sensitive_state: { ...(data.option.sensitive_state || {}) }
    })
    drafts[option.key] = { ...(data.option.value || {}) }
    message.success('配置已保存')
  } catch (error) {
    message.error(error.message || '保存配置失败')
  } finally {
    savingOption.value = ''
  }
}

onMounted(loadConfigOptions)
</script>

<style lang="less" scoped>
.settings-panel {
  background-color: var(--gray-50);
  border: 1px solid var(--gray-200);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-title {
  margin: 0 0 10px;
  color: var(--gray-900);
  font-size: 15px;
  font-weight: 600;

  &:not(:first-child) {
    margin-top: 24px;
  }
}

.section-description {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.card {
  display: flex;
  align-items: center;
  justify-content: space-between;

  .label {
    margin-right: 20px;
    font-weight: 500;
    color: var(--gray-800);
    flex-shrink: 0;
    min-width: 140px;
  }

  &.card-select {
    align-items: flex-start;
    gap: 12px;

    .label {
      margin-right: 0;
      margin-top: 6px;
    }
  }
}

// 配置项控件容器：与 label 对齐，内部控件统一宽度、右侧对齐
.setting-control {
  flex: 1;
  max-width: 480px;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.ocr-option {
  .option-meta {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 140px;
    flex-shrink: 0;

    .label {
      font-weight: 500;
      color: var(--gray-800);
    }

    .option-desc {
      color: var(--color-text-secondary);
      font-size: 12px;
      line-height: 1.5;
    }
  }
}

.option-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 16px;
}

.option-field {
  display: flex;
  flex-direction: column;
  gap: 6px;

  &:only-child {
    grid-column: 1 / -1;
  }

  small {
    color: var(--color-text-secondary);
    font-size: 12px;
    line-height: 1.5;
  }
}

.setting-label {
  color: var(--gray-700);
  font-size: 13px;
  font-weight: 500;
}

.save-button {
  border-color: var(--gray-900);
  background: var(--gray-900);
  margin-top: 4px;
  align-self: flex-start;

  &:hover,
  &:focus {
    border-color: var(--gray-700);
    background: var(--gray-700);
  }
}

@media (max-width: 680px) {
  .option-fields {
    grid-template-columns: 1fr;
  }

  .option-field:only-child {
    grid-column: auto;
  }
}
</style>
