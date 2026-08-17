import { apiGet } from './base'

/**
 * Dashboard API模块
 * 用于管理员查看所有对话记录
 */

export const dashboardApi = {
  /**
   * 获取所有对话记录
   * @param {Object} params - 查询参数
   * @param {string} params.uid - 用户 UID 过滤
   * @param {string} params.agent_id - 智能体ID过滤
   * @param {string} params.status - 状态过滤 (active/deleted/all)
   * @param {number} params.limit - 每页数量
   * @param {number} params.offset - 偏移量
   * @returns {Promise<Array>} - 对话列表
   */
  getConversations: (params = {}) => {
    const queryParams = new URLSearchParams()
    if (params.uid) queryParams.append('uid', params.uid)
    if (params.agent_id) queryParams.append('agent_id', params.agent_id)
    if (params.status) queryParams.append('status', params.status)
    if (params.limit) queryParams.append('limit', params.limit)
    if (params.offset) queryParams.append('offset', params.offset)

    return apiGet(`/api/dashboard/conversations?${queryParams.toString()}`)
  },

  /**
   * 获取对话详情
   * @param {string} threadId - 对话线程ID
   * @returns {Promise<Object>} - 对话详情
   */
  getConversationDetail: (threadId) => {
    return apiGet(`/api/dashboard/conversations/${threadId}`)
  },

  /**
   * 获取Dashboard基础统计信息
   * @returns {Promise<Object>} - 统计信息
   */
  getStats: () => {
    return apiGet('/api/dashboard/stats')
  },

  /**
   * 获取用户反馈列表
   * @param {Object} params - 查询参数
   * @param {string} params.rating - 反馈类型过滤 (like/dislike/all)
   * @param {string} params.agent_id - 智能体ID过滤
   * @returns {Promise<Array>} - 反馈列表
   */
  getFeedbacks: (params = {}) => {
    const queryParams = new URLSearchParams()
    if (params.rating && params.rating !== 'all') queryParams.append('rating', params.rating)
    if (params.agent_id) queryParams.append('agent_id', params.agent_id)

    return apiGet(`/api/dashboard/feedbacks?${queryParams.toString()}`)
  },

  /**
   * 获取调用统计时间序列数据
   * @param {string} type - 数据类型 (models/agents/tokens/tools)
   * @param {string} timeRange - 时间范围 (14hours/14days/14weeks)
   * @returns {Promise<Object>} - 时间序列统计数据
   */
  getCallTimeseries: (type = 'models', timeRange = '14days') => {
    return apiGet(`/api/dashboard/stats/calls/timeseries?type=${type}&time_range=${timeRange}`)
  },

  /**
   * 仅获取基础统计数据（无需并行请求）
   * @returns {Promise<Object>} - 基础统计数据
   */
  getAllStats: async () => {
    try {
      const basicStats = await apiGet('/api/dashboard/stats')
      return { basic: basicStats }
    } catch (error) {
      console.error('获取基础统计数据失败:', error)
      throw error
    }
  }
}