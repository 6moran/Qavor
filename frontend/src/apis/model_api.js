import { apiDelete, apiGet, apiPost, apiPut } from './base.js'

export const buildModelQuery = ({ page, page_size, keyword, model_type } = {}) => {
  const params = new URLSearchParams()
  if (page) params.set('page', page)
  if (page_size) params.set('page_size', page_size)
  if (keyword?.trim()) params.set('keyword', keyword.trim())
  if (model_type) params.set('model_type', model_type)
  const query = params.toString()
  return query ? `?${query}` : ''
}

export const unwrapModelList = (response) => response?.data || { total: 0, items: [] }

// 并发去重的「已启用模型列表」获取：
// 同一 model_type 在同一时刻的多次调用共享同一次请求（多个选择器实例挂载时同时预加载，
// 避免每个实例各发一次请求）；请求完成后立即释放缓存，保证下次挂载能拿到最新的模型数据。
const inflightModelLists = new Map()

export const getEnabledModels = async (modelType, { page = 1, page_size = 100 } = {}) => {
  const key = `${modelType}|${page}|${page_size}`
  if (!inflightModelLists.has(key)) {
    const promise = modelApi
      .list({ model_type: modelType, page, page_size })
      .then((response) => (response?.data?.items || []).filter((model) => model.enabled))
      .finally(() => inflightModelLists.delete(key))
    inflightModelLists.set(key, promise)
  }
  return inflightModelLists.get(key)
}

export const modelApi = {
  list: (params = {}) => apiGet(`/api/v1/models${buildModelQuery(params)}`),
  get: (id) => apiGet(`/api/v1/models/${encodeURIComponent(id)}`),
  create: (payload) => apiPost('/api/v1/models', payload),
  update: (id, payload) => apiPut(`/api/v1/models/${encodeURIComponent(id)}`, payload),
  remove: (id) => apiDelete(`/api/v1/models/${encodeURIComponent(id)}`),
  testConnection: (payload) => apiPost('/api/v1/models/test', payload)
}
