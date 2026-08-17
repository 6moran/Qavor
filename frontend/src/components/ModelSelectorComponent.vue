<template>
  <a-dropdown
    trigger="click"
    :open="dropdownOpen"
    :disabled="props.disabled"
    destroy-popup-on-hide
    @open-change="handleOpenChange"
  >
    <div ref="triggerRef" class="model-select" :class="modelSelectClasses" @click.prevent.stop>
      <span class="model-text">{{ displayModelText }}</span>
      <button
        v-if="props.clearable && props.model_spec && !props.disabled"
        class="model-clear-btn"
        @click.stop="handleClear"
      >
        ×
      </button>
    </div>
    <template #overlay>
      <div class="model-dropdown" :style="overlayStyle" @click.stop>
        <a-input v-model:value="keyword" placeholder="搜索模型" allow-clear />
        <a-menu class="scrollable-menu">
          <a-menu-item v-if="loading" key="loading" disabled>加载中...</a-menu-item>
          <a-menu-item v-else-if="!filteredModels.length" key="empty" disabled>
            暂无可用模型
          </a-menu-item>
          <a-menu-item
            v-for="model in filteredModels"
            v-else
            :key="model.id"
            @click="handleSelectModel(String(model.id))"
          >
            <div class="model-option">
              <span>{{ model.remark || model.name }}</span>
              <span class="model-type">{{ model.model_type }}</span>
            </div>
          </a-menu-item>
        </a-menu>
        <div class="model-config-link">
          没有合适的模型？
          <RouterLink :to="{ path: '/agent-manage', query: { tab: 'providers' } }" @click.stop>
            配置模型
          </RouterLink>
        </div>
      </div>
    </template>
  </a-dropdown>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { getEnabledModels } from '@/apis/model_api'
import { modelApi } from '@/apis/model_api'
import { useDropdownWidth } from '@/composables/useDropdownWidth'

const dropdownOpen = ref(false)
const { triggerRef, overlayStyle } = useDropdownWidth(dropdownOpen, 280)

const props = defineProps({
  model_spec: { type: String, default: '' },
  placeholder: { type: String, default: '请选择模型' },
  size: { type: String, default: 'small' },
  disabled: { type: Boolean, default: false },
  clearable: { type: Boolean, default: false },
  displayName: { type: String, default: 'full' }
})

const emit = defineEmits(['select-model', 'update:model_spec'])
const models = ref([])
const keyword = ref('')
const loading = ref(false)
let loaded = false

const filteredModels = computed(() => {
  const value = keyword.value.trim().toLowerCase()
  if (!value) return models.value
  return models.value.filter((model) => `${model.name} ${model.remark || ''} ${model.protocol}`.toLowerCase().includes(value))
})

const modelSelectClasses = computed(() => ({
  'model-select--middle': props.size === 'middle',
  'model-select--large': props.size === 'large',
  'model-select--disabled': props.disabled
}))

const displayModelText = computed(() => {
  if (!props.model_spec) return props.placeholder
  const model = models.value.find((item) => String(item.id) === props.model_spec)
  return model?.remark || model?.name || props.model_spec
})

const loadModels = async () => {
  if (loaded) return
  loading.value = true
  try {
    models.value = await getEnabledModels('chat')
    loaded = true
  } catch (error) {
    console.error('获取 chat 模型失败:', error)
  } finally {
    loading.value = false
  }
}

const handleOpenChange = async (open) => {
  dropdownOpen.value = open
  if (open) {
    await loadModels()
  }
}

// 挂载时若有已选模型则预加载模型列表，让选中值直接显示模型名而非数字 id；
// 列表为空时仍保持展开下拉才加载的懒加载行为
onMounted(() => {
  if (props.model_spec) {
    loadModels()
  }
})

watch(() => props.model_spec, (newVal) => {
  if (newVal && !loaded) {
    loadModels()
  }
})

const handleSelectModel = (id) => {
  emit('select-model', id)
  emit('update:model_spec', id)
  dropdownOpen.value = false
}

const handleClear = () => {
  emit('select-model', '')
  emit('update:model_spec', '')
}

onBeforeUnmount(() => {
  dropdownOpen.value = false
})
</script>

<style lang="less" scoped>
@import '@/assets/css/model-selector-common.less';

.model-select { display: flex; align-items: center; justify-content: space-between; }
.model-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.model-clear-btn { margin-left: 8px; border: 0; background: transparent; cursor: pointer; }
.model-dropdown { padding: 8px; }
.model-option { display: flex; justify-content: space-between; gap: 12px; }
.model-type { color: var(--gray-500); font-size: 11px; }
.model-config-link { padding: 8px 4px 2px; color: var(--gray-500); font-size: 11px; }
.model-config-link a { color: var(--main-600); }
</style>
