<template>
  <a-select
    :value="props.value"
    :placeholder="props.placeholder"
    :size="props.size"
    :style="props.style"
    :disabled="props.disabled"
    :loading="loading"
    @dropdown-visible-change="handleOpenChange"
    @change="handleSelect"
  >
    <a-select-option v-for="model in models" :key="model.id" :value="String(model.id)">
      {{ model.name }}
    </a-select-option>
  </a-select>
</template>

<script setup>
import { ref } from 'vue'
import { modelApi } from '@/apis/model_api'

const props = defineProps({
  value: { type: [String, Number], default: '' },
  placeholder: { type: String, default: '请选择重排序模型' },
  size: { type: String, default: 'small' },
  style: { type: Object, default: () => ({ width: '100%' }) },
  disabled: { type: Boolean, default: false }
})

const emit = defineEmits(['update:value', 'change'])
const models = ref([])
const loading = ref(false)
let loaded = false

const handleOpenChange = async (open) => {
  if (!open || loaded) return
  loading.value = true
  try {
    const response = await modelApi.list({ model_type: 'rerank', page: 1, page_size: 100 })
    models.value = (response?.data?.items || []).filter((model) => model.enabled)
    loaded = true
  } catch (error) {
    console.error('获取 rerank 模型失败:', error)
  } finally {
    loading.value = false
  }
}

const handleSelect = (value) => {
  emit('update:value', value)
  emit('change', value)
}
</script>
