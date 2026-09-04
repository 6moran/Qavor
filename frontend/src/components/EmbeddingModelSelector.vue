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
      {{ model.remark || model.name }}
    </a-select-option>
  </a-select>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { getEnabledModels } from '@/apis/model_api'

const props = defineProps({
  value: { type: [String, Number], default: '' },
  placeholder: { type: String, default: '请选择嵌入模型' },
  size: { type: String, default: 'small' },
  style: { type: Object, default: () => ({ width: '100%' }) },
  disabled: { type: Boolean, default: false }
})

const emit = defineEmits(['update:value', 'change'])
const models = ref([])
const loading = ref(false)
let loaded = false

const loadModels = async () => {
  loading.value = true
  try {
    models.value = await getEnabledModels('embedding')
    loaded = true
  } catch (error) {
    console.error('获取 embedding 模型失败:', error)
  } finally {
    loading.value = false
  }
}

const handleOpenChange = async (open) => {
  if (!open || loaded) return
  await loadModels()
}

const handleSelect = (value) => {
  emit('update:value', value)
  emit('change', value)
}

// 挂载时若有已选模型则预加载，避免选中值直接显示数字 id
onMounted(() => {
  if (props.value) loadModels()
})

// 配置是异步流入的（如 configStore 晚于挂载才填充），此时 onMounted 时 value 还为空、
// 不会触发预加载，列表永远为空，a-select 会把 id 当文本直接显示。
// 监听 value：一旦被父组件赋值即补加载模型列表，确保 id 能解析成名字。
watch(
  () => props.value,
  (val) => {
    if (val && !loaded) loadModels()
  }
)
</script>
