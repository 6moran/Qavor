import { apiGet } from './base'

/**
 * 链路追踪 API 模块
 * 提供 Agent 对话执行链路（Trace）的查询能力
 */

const BASE_URL = '/api/v1/traces'
const RUNS_URL = '/api/v1/runs'

export const traceApi = {
  /**
   * Trace 列表（分页 + 筛选）
   * @param {Object} params - keyword / agent_slug / conversation_id / status / model / tool / error_only / mismatch_only / from / to / page / page_size
   * @returns {Promise} - { items, total }
   */
  listTraces: (params = {}) => apiGet(BASE_URL, { params }).then(res => res?.data || { items: [], total: 0 }),

  /**
   * Trace 详情（头部 + spans 平铺 + diagnostics）
   * @param {string} traceId
   * @returns {Promise} - { trace, run, spans, diagnostics }
   */
  getTrace: (traceId) => apiGet(`${BASE_URL}/${traceId}`).then(res => res?.data || null),

  /**
   * 通过 run_id 反查 trace_id
   * @param {string} runId
   * @returns {Promise} - { trace_id }
   */
  getTraceByRunId: (runId) => apiGet(`${RUNS_URL}/${encodeURIComponent(runId)}/trace`).then(res => res?.data || null)
}

export default traceApi
