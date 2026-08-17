import { apiGet, apiPost, apiPut, apiDelete } from './base'

/**
 * MCP 服务器管理 API 模块
 * 包含 MCP 服务器的增删改查和工具管理功能
 */

const BASE_URL = '/api/v1/mcp'

// =============================================================================
// === MCP 服务器 CRUD ===
// =============================================================================

/**
 * 获取所有 MCP 服务器配置
 * @returns {Promise} - 服务器列表
 */
export const getMcpServers = async () => {
  // 后端列表路由为 /mcp/list，响应为 { code, data: { list, total, ... } }
  const result = await apiGet(`${BASE_URL}/list`)
  if (result?.data?.list) {
    return { success: result.code === 0, data: result.data.list, message: result.message }
  }
  return { success: result?.code === 0, data: result?.data || [], message: result?.message }
}

/**
 * 获取单个 MCP 服务器配置
 * @param {string} name - 服务器名称
 * @returns {Promise} - 服务器配置
 */
export const getMcpServer = async (name) => {
  // 后端响应为 { code, data: MCPServerResponse }
  const result = await apiGet(`${BASE_URL}/${encodeURIComponent(name)}`)
  if (result?.data) {
    return { success: result.code === 0, data: result.data, message: result.message }
  }
  return { success: result?.code === 0, data: null, message: result?.message }
}

/**
 * 创建新的 MCP 服务器
 * @param {Object} data - 服务器配置数据
 * @returns {Promise} - 创建结果
 */
export const createMcpServer = async (data) => {
  const result = await apiPost(BASE_URL, data)
  return { success: result?.code === 0, data: result?.data, message: result?.message }
}

/**
 * 更新 MCP 服务器配置
 * @param {string} name - 服务器名称
 * @param {Object} data - 更新数据
 * @returns {Promise} - 更新结果
 */
export const updateMcpServer = async (name, data) => {
  const result = await apiPut(`${BASE_URL}/${encodeURIComponent(name)}`, data)
  return { success: result?.code === 0, data: result?.data, message: result?.message }
}

/**
 * 删除 MCP 服务器
 * @param {string} name - 服务器名称
 * @returns {Promise} - 删除结果
 */
export const deleteMcpServer = async (name) => {
  const result = await apiDelete(`${BASE_URL}/${encodeURIComponent(name)}`)
  return { success: result?.code === 0, data: result?.data, message: result?.message }
}

// =============================================================================
// === MCP 服务器操作 ===
// =============================================================================

/**
 * 测试 MCP 服务器连接
 * @param {string} name - 服务器名称
 * @returns {Promise} - 测试结果
 */
export const testMcpServer = async (name) => {
  const result = await apiPost(`${BASE_URL}/${encodeURIComponent(name)}/test`, {})
  return { success: result?.code === 0, data: result?.data, message: result?.message }
}

/**
 * 测试 MCP 配置连接（用于添加/编辑表单的预验证）
 * @param {Object} data - 表单配置数据
 * @returns {Promise} - 测试结果
 */
export const testMcpServerConfig = async (data) => {
  const result = await apiPost(`${BASE_URL}/test`, data)
  return { success: result?.code === 0, data: result?.data, message: result?.message }
}

/**
 * 更新 MCP 服务器启用状态
 * @param {string} name - 服务器名称
 * @param {boolean} enabled - 是否启用
 * @returns {Promise} - 切换结果
 */
export const updateMcpServerStatus = async (name, enabled) => {
  // 后端使用独立的 /enable 和 /disable 端点，均为 POST 方法
  const action = enabled ? 'enable' : 'disable'
  const result = await apiPost(`${BASE_URL}/${encodeURIComponent(name)}/${action}`)
  return { success: result?.code === 0, data: result?.data, message: result?.message }
}

// =============================================================================
// === MCP 工具管理 ===
// =============================================================================

/**
 * 获取 MCP 服务器的工具列表
 * @param {string} name - 服务器名称
 * @returns {Promise} - 工具列表
 */
export const getMcpServerTools = async (name) => {
  // 后端响应为 { code, data: { tools: [...] } }
  const result = await apiGet(`${BASE_URL}/${encodeURIComponent(name)}/tools`)
  if (result?.data?.tools) {
    return { success: result.code === 0, data: result.data.tools, message: result.message }
  }
  return { success: result?.code === 0, data: result?.data || [], message: result?.message }
}

/**
 * 刷新 MCP 服务器的工具列表（清除缓存重新获取）
 * @param {string} name - 服务器名称
 * @returns {Promise} - 刷新结果
 */
export const refreshMcpServerTools = async (name) => {
  // 后端响应为 { code, data: { tools: [...] } }
  const result = await apiPost(`${BASE_URL}/${encodeURIComponent(name)}/tools/refresh`, {})
  if (result?.data?.tools) {
    return { success: result.code === 0, data: result.data.tools, message: result.message }
  }
  return { success: result?.code === 0, data: result?.data || [], message: result?.message }
}

/**
 * 切换单个工具的启用状态
 * @param {string} serverName - 服务器名称
 * @param {string} toolName - 工具名称
 * @returns {Promise} - 切换结果
 */
export const toggleMcpServerTool = async (serverName, toolName) => {
  const result = await apiPut(
    `${BASE_URL}/${encodeURIComponent(serverName)}/tools/${encodeURIComponent(toolName)}/toggle`,
    {}
  )
  return { success: result?.code === 0, data: result?.data, message: result?.message }
}

export const mcpApi = {
  getMcpServers,
  getMcpServer,
  createMcpServer,
  updateMcpServer,
  deleteMcpServer,
  testMcpServer,
  testMcpServerConfig,
  updateMcpServerStatus,
  getMcpServerTools,
  refreshMcpServerTools,
  toggleMcpServerTool
}

export default mcpApi
