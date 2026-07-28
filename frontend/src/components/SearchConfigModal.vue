<template>
  <a-modal
    :open="props.modelValue"
    title="检索配置"
    width="800px"
    :confirm-loading="saving"
    @ok="handleSave"
    @cancel="handleCancel"
    ok-text="保存"
    cancel-text="取消"
  >
    <SearchConfigPanel
      v-if="props.modelValue"
      ref="searchConfigPanelRef"
      :kb-id="props.kbId"
      @save="handlePanelSave"
    />
  </a-modal>
</template>

<script setup>
import { ref } from 'vue'
import SearchConfigPanel from '@/components/SearchConfigPanel.vue'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  kbId: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:modelValue', 'save'])

const searchConfigPanelRef = ref(null)
const saving = ref(false)

const handleSave = async () => {
  if (!searchConfigPanelRef.value) return
  saving.value = true
  try {
    const success = await searchConfigPanelRef.value.save()
    if (success) {
      emit('update:modelValue', false)
    }
  } finally {
    saving.value = false
  }
}

const handlePanelSave = (config) => {
  emit('save', config)
  emit('update:modelValue', false)
}

const handleCancel = () => {
  emit('update:modelValue', false)
}
</script>
