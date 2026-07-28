<template>
  <div class="basic-settings-section">
    <template v-if="userStore.isAdmin">
      <template v-if="userStore.isSuperAdmin">
        <div class="section-title">默认项配置</div>
        <div class="settings-panel">
          <div class="setting-row two-cols">
            <div class="col-item">
              <div class="setting-label">{{ items?.default_model?.des || '默认对话模型' }}</div>
              <div class="setting-content">
                <ModelSelectorComponent
                  @select-model="handleChatModelSelect"
                  :model_spec="configStore.config?.default_model"
                  placeholder="请选择默认模型"
                />
              </div>
            </div>
            <div class="col-item">
              <div class="setting-label">{{ items?.fast_model?.des }}</div>
              <div class="setting-content">
                <ModelSelectorComponent
                  @select-model="handleFastModelSelect"
                  :model_spec="configStore.config?.fast_model"
                  placeholder="请选择模型"
                />
              </div>
            </div>
          </div>
          <div class="setting-row two-cols">
            <div class="col-item">
              <div class="setting-label">{{ items?.embed_model?.des }}</div>
              <div class="setting-content">
                <EmbeddingModelSelector
                  :value="configStore.config?.embed_model"
                  @change="handleChange('embed_model', $event)"
                  style="width: 100%"
                />
              </div>
            </div>
            <div class="col-item">
              <div class="setting-label">{{ items?.reranker?.des }}</div>
              <div class="setting-content">
                <RerankModelSelector
                  :value="configStore.config?.reranker"
                  @change="handleChange('reranker', $event)"
                  style="width: 100%"
                />
              </div>
            </div>
          </div>
        </div>
      </template>

      <template v-if="userStore.isSuperAdmin">
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
      </template>
    </template>

    
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useConfigStore } from '@/stores/config'
import { useUserStore } from '@/stores/user'
import ModelSelectorComponent from '@/components/ModelSelectorComponent.vue'
import EmbeddingModelSelector from '@/components/EmbeddingModelSelector.vue'
import RerankModelSelector from '@/components/RerankModelSelector.vue'

const configStore = useConfigStore()
const userStore = useUserStore()
const items = computed(() => configStore.config?._config_items || {})
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
</script>

<style lang="less" scoped>
.basic-settings-section {
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

  .setting-row {
    display: flex;
    flex-direction: column;
    gap: 8px;

    &.two-cols {
      flex-direction: row;
      gap: 20px;
    }

    .col-item {
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 6px;
      min-width: 0;
    }
  }

  .setting-label {
    font-size: 13px;
    font-weight: 500;
    color: var(--gray-700);
  }

  .setting-content {
    width: 100%;

    .full-width {
      width: 100%;
    }
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
