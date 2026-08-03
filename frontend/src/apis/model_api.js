import { apiAdminDelete, apiAdminPost, apiAdminPut, apiGet } from './base.js'

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

export const modelApi = {
  list: (params = {}) => apiGet(`/api/v1/models${buildModelQuery(params)}`),
  get: (id) => apiGet(`/api/v1/models/${encodeURIComponent(id)}`),
  create: (payload) => apiAdminPost('/api/v1/models', payload),
  update: (id, payload) => apiAdminPut(`/api/v1/models/${encodeURIComponent(id)}`, payload),
  remove: (id) => apiAdminDelete(`/api/v1/models/${encodeURIComponent(id)}`)
}
