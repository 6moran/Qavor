import { apiDelete, apiGet, apiPost } from './base'

const BASE_URL = '/api/tasks'

export const taskerApi = {
  fetchTasks: async (params = {}) => {
    const query = new URLSearchParams(params).toString()
    const url = query ? `${BASE_URL}?${query}` : BASE_URL
    return apiGet(url)
  },

  fetchTaskDetail: async (taskId) => {
    return apiGet(`${BASE_URL}/${taskId}`)
  },

  cancelTask: async (taskId) => {
    return apiPost(`${BASE_URL}/${taskId}/cancel`, {})
  },

  deleteTask: async (taskId) => {
    return apiDelete(`${BASE_URL}/${taskId}`)
  }
}
