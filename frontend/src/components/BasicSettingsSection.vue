<template>
  <div class="basic-settings-section">
    <div class="section-title">默认项配置</div>
        <div class="settings-panel">
          <div class="card card-select">
            <span class="label">{{ items?.default_model?.des || '默认对话模型' }}</span>
            <div class="setting-control">
              <ModelSelectorComponent
                @select-model="handleChatModelSelect"
                :model_spec="configStore.config?.default_model"
                placeholder="请选择默认模型"
              />
            </div>
          </div>
          <div class="card card-select">
            <span class="label">{{ items?.fast_model?.des || '快速对话模型' }}</span>
            <div class="setting-control">
              <ModelSelectorComponent
                @select-model="handleFastModelSelect"
                :model_spec="configStore.config?.fast_model"
                placeholder="请选择模型"
              />
            </div>
          </div>
          <div class="card card-select">
            <span class="label">{{ items?.embed_model?.des || '嵌入模型' }}</span>
            <div class="setting-control">
              <EmbeddingModelSelector
                :value="configStore.config?.embed_model"
                @change="handleChange('embed_model', $event)"
              />
            </div>
          </div>
          <div class="card card-select">
            <span class="label">{{ items?.reranker?.des || '重排序模型' }}</span>
            <div class="setting-control">
              <RerankModelSelector
                :value="configStore.ragSettings.rerankModelId"
                :loading="ragSettingsLoading"
                @change="handleRerankModelChange"
              />
              <div class="setting-hint">
                {{ configStore.ragSettings.rerankModelName || '未配置，将使用 RRF 融合结果' }}
              </div>
            </div>
          </div>
        </div>

      <div class="section-title">内容审查配置</div>
        <div class="section">
          <div class="card">
            <span class="label">{{ items?.enable_content_guard?.des }}</span>
            <a-switch
              :checked="configStore.config?.enable_content_guard"
              @change="handleChange('enable_content_guard', $event)"
            />
          </div>
          <div class="card" v-if="configStore.config?.enable_content_guard">
            <span class="label">{{ items?.enable_content_guard_llm?.des }}</span>
            <a-switch
              :checked="configStore.config?.enable_content_guard_llm"
              @change="handleChange('enable_content_guard_llm', $event)"
            />
          </div>
          <div
            class="card card-select"
            v-if="
              configStore.config?.enable_content_guard &&
              configStore.config?.enable_content_guard_llm
            "
          >
            <span class="label">{{ items?.content_guard_llm_model?.des }}</span>
            <ModelSelectorComponent
              @select-model="handleContentGuardModelSelect"
              :model_spec="configStore.config?.content_guard_llm_model"
              placeholder="请选择模型"
            />
          </div>
        </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useConfigStore } from '@/stores/config'
import ModelSelectorComponent from '@/components/ModelSelectorComponent.vue'
import EmbeddingModelSelector from '@/components/EmbeddingModelSelector.vue'
import RerankModelSelector from '@/components/RerankModelSelector.vue'

const configStore = useConfigStore()
const items = computed(() => configStore.config?._config_items || {})
const ragSettingsLoading = ref(false)
const handleChange = (key, e) => {
  configStore.setConfigValue(key, e)
}

const handleChatModelSelect = (spec) => {
  if (typeof spec === 'string' && spec) {
    configStore.setConfigValue('default_model', spec)
  }
}

const handleFastModelSelect = (spec) => {
  if (typeof spec === 'string' && spec) {
    configStore.setConfigValue('fast_model', spec)
  }
}

const handleContentGuardModelSelect = (spec) => {
  if (typeof spec === 'string' && spec) {
    configStore.setConfigValue('content_guard_llm_model', spec)
  }
}

const loadRagSettings = async () => {
  ragSettingsLoading.value = true
  try {
    await configStore.refreshRagSettings()
  } catch (error) {
    message.error(error.message || '加载 Rerank 设置失败')
  } finally {
    ragSettingsLoading.value = false
  }
}

const handleRerankModelChange = async (modelId) => {
  ragSettingsLoading.value = true
  try {
    await configStore.updateRerankModel(modelId)
    message.success(modelId ? '全局 Rerank 模型已更新' : '已关闭全局 Rerank')
  } catch (error) {
    message.error(error.message || '保存 Rerank 设置失败，已恢复原值')
  } finally {
    ragSettingsLoading.value = false
  }
}

onMounted(loadRagSettings)
</script>

<style lang="less" scoped>
.basic-settings-section {
  // 统一下拉框高度：自定义模型选择器与 antd small select(24px) 对齐，避免两列高低不齐
  :deep(.model-select) {
    height: 24px;
    padding: 0 8px;
    box-sizing: border-box;
  }

  .section {
    background-color: var(--gray-0);
    padding: 10px 16px;
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    border: 1px solid var(--gray-150);
  }

  .settings-panel {
    background-color: var(--gray-50);
    border: 1px solid var(--gray-200);
    border-radius: 8px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .setting-hint {
    margin-top: 5px;
    color: var(--gray-500);
    font-size: 11px;
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

  .agent-select {
    width: 320px;
    max-width: 100%;
  }

  .services-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 12px;
    margin-top: 16px;
  }

  .service-link-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 16px;
    border: 1px solid var(--gray-150);
    border-radius: 8px;
    background: var(--gray-0);
    transition: all 0.2s;
    min-height: 70px;

    &:hover {
      box-shadow: 0 1px 8px var(--gray-150);
      border-color: var(--gray-100);
    }

    .service-info {
      flex: 1;
      margin-right: 16px;

      h4 {
        margin: 0 0 4px 0;
        color: var(--gray-900);
        font-size: 15px;
        font-weight: 500;
      }

      p {
        margin: 0;
        color: var(--gray-600);
        font-size: 13px;
        line-height: 1.4;
      }
    }
  }

  @media (max-width: 768px) {
    .agent-select {
      width: 100%;
    }
  }
}
</style>
