import { apiGet, apiPost, apiPut, apiDelete } from './base'
import { buildRagSettingsPayload } from '@/utils/rag_settings'

/**
 * 系统管理API模块
 * 包含系统配置、健康检查、信息管理等功能
 */

// =============================================================================
// === 健康检查分组 ===
// =============================================================================

export const healthApi = {
  /**
   * 系统健康检查（公开接口）
   * @returns {Promise} - 健康检查结果
   */
  checkHealth: () => {
    return apiGet('/api/v1/health', {}, false)
  }
}

// =============================================================================
// === 配置管理分组 ===
// =============================================================================

export const configApi = {
  /**
   * 获取系统配置
   * @returns {Promise} - 系统配置（已解包 response.Success 的 data）
   */
  getConfig: async () => {
    const data = await apiGet('/api/system/config')
    return data?.data ?? data
  },

  /**
   * 更新单个配置项
   * @param {string} key - 配置键
   * @param {any} value - 配置值
   * @returns {Promise} - 更新结果（已解包 data）
   */
  updateConfig: async (key, value) => {
    const data = await apiPost('/api/system/config', { key, value })
    return data?.data ?? data
  },

  /**
   * 批量更新配置项
   * @param {Object} items - 配置项对象
   * @returns {Promise} - 更新结果（已解包 data）
   */
  updateConfigBatch: async (items) => {
    const data = await apiPost('/api/system/config/update', items)
    return data?.data ?? data
  },

  /**
   * 获取系统日志
   * @param {string} levels - 可选的日志级别过滤，多个级别用逗号分隔
   * @returns {Promise} - 系统日志
   */
  getLogs: async (levels) => {
    const url = levels
      ? `/api/system/logs?levels=${encodeURIComponent(levels)}`
      : '/api/system/logs'
    return apiGet(url)
  }
}

export const configOptionsApi = {
  getOptions: async () => {
    const data = await apiGet('/api/system/config/options')
    return data?.data ?? data
  },

  updateOption: async (key, value) => {
    const data = await apiPut(`/api/system/config/options/${encodeURIComponent(key)}`, { value })
    return data?.data ?? data
  }
}

export const ragSettingsApi = {
  getRagSettings: () => apiGet('/api/system/rag-settings'),
  updateRagSettings: (rerankModelId) =>
    apiPut('/api/system/rag-settings', buildRagSettingsPayload(rerankModelId))
}

// =============================================================================
// === 信息管理分组 ===
// =============================================================================

export const brandApi = {
  /**
   * 获取系统信息配置（公开接口）
   * @returns {Promise} - 系统信息配置
   */
  getInfoConfig: () => {
    return apiGet('/api/system/info', {}, false)
  }
}

// =============================================================================
// === OCR服务分组 ===
// =============================================================================

export const ocrApi = {
  getOptions: async () => apiGet('/api/system/ocr/options'),
  getHealth: async () => apiGet('/api/system/ocr/health')
}

// =============================================================================
// === 聊天模型状态检查分组 ===
// =============================================================================

export const chatModelApi = {}

// =============================================================================
// === 独立模型供应商配置分组 ===
// =============================================================================

export const modelProviderApi = {
  getProviders: async () => {
    return apiGet('/api/system/model-providers')
  },

  getV2Models: async (modelType = 'chat') => {
    return apiGet(`/api/system/model-providers/models/v2?model_type=${modelType}`)
  },

  refreshModelCache: async () => {
    return apiPost('/api/system/model-providers/models/cache/refresh')
  },

  getModelStatusBySpec: async (spec) => {
    return apiGet(`/api/system/model-providers/models/status?spec=${encodeURIComponent(spec)}`)
  },

  createProvider: async (payload) => {
    return apiPost('/api/system/model-providers', payload)
  },

  updateProvider: async (providerId, payload) => {
    return apiPut(`/api/system/model-providers/${encodeURIComponent(providerId)}`, payload)
  },

  deleteProvider: async (providerId) => {
    return apiDelete(`/api/system/model-providers/${encodeURIComponent(providerId)}`)
  },

  fetchRemoteModels: async (providerId) => {
    return apiGet(
      `/api/system/model-providers/${encodeURIComponent(providerId)}/remote-models`
    )
  }
}
