import { ref } from 'vue'
import { defineStore } from 'pinia'
import { configApi, ragSettingsApi } from '@/apis/system_api'
import {
  normalizeRagSettingsResponse,
  persistRerankSelection
} from '@/utils/rag_settings'

export const useConfigStore = defineStore('config', () => {
  const config = ref({})
  const ragSettings = ref({ rerankModelId: null, rerankModelName: '' })
  function setConfig(newConfig) {
    config.value = newConfig
  }

  function setConfigValue(key, value) {
    config.value[key] = value
    configApi.updateConfigBatch({ [key]: value }).then((data) => {
      console.debug('Success:', data)
      setConfig(data)
    })
  }

  async function refreshConfig() {
    const data = await configApi.getConfig()
    console.log('config', data)
    setConfig(data)
    return data
  }

  async function refreshRagSettings() {
    const response = await ragSettingsApi.getRagSettings()
    ragSettings.value = normalizeRagSettingsResponse(response)
    return ragSettings.value
  }

  async function updateRerankModel(modelId) {
    const previous = { ...ragSettings.value }
    const result = await persistRerankSelection({
      previous,
      nextModelId: modelId,
      update: ragSettingsApi.updateRagSettings
    })
    ragSettings.value = result.settings
    if (result.error) throw result.error
    return ragSettings.value
  }

  return {
    config,
    ragSettings,
    setConfigValue,
    refreshConfig,
    refreshRagSettings,
    updateRerankModel
  }
})
