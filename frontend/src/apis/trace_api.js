import { apiGet } from './base'

/**
 * 链路追踪 API 模块
 * 提供 Agent 对话执行链路（Trace）的查询能力
 */

const BASE_URL = '/api/v1/traces'

export const traceApi = {
  /**
   * Trace 列表（分页 + 筛选）
   * @param {Object} params - keyword / agent_slug / status / source / from / to / page / page_size
   * @returns {Promise} - { items, total }
   */
  listTraces: (params = {}) => apiGet(BASE_URL, { params }).then(res => res?.data || { items: [], total: 0 }),

  /**
   * Trace 详情（头部 + spans 平铺）
   * @param {string} traceId
   * @returns {Promise} - { trace, spans }
   */
  getTrace: (traceId) => apiGet(`${BASE_URL}/${traceId}`).then(res => res?.data || null)
}

export default traceApi
