import { apiGet, apiDelete, apiPost, apiPut } from './base'

/**
 * 解包 Go 后端统一响应信封 {code, message, data}，返回 data。
 * code !== 0 时抛错。
 */
const unwrap = (response) => {
  if (response && typeof response === 'object' && 'code' in response) {
    if (response.code !== 0) {
      throw new Error(response.message || '请求失败')
    }
    return response.data
  }
  return response
}

const buildQuery = (params) => {
  const query = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value))
    }
  })
  return query.toString()
}

/**
 * 构建带 agent slug 前缀的路径：path = <slug>/<userPath>
 */
const buildAgentPath = (slug, userPath) => {
  const cleanPath = String(userPath || '')
    .replace(/^\/+/, '')
    .replace(/\/+$/, '')
  if (!cleanPath) return slug
  // 如果路径已经包含 slug 前缀，不加（防止双前缀）
  if (cleanPath === slug || cleanPath.startsWith(slug + '/')) {
    return cleanPath
  }
  return `${slug}/${cleanPath}`
}

/**
 * 获取文件系统树
 * @param {string} agentSlug - agent slug
 * @param {string} path - 路径（相对于 agent 工作区根）
 * @returns {Promise<{entries: Array}>}
 */
export const getViewerFileSystemTree = async (agentSlug, path = '/') => {
  const agentPath = buildAgentPath(agentSlug, path)
  const response = await apiGet(`/api/workspace/tree?${buildQuery({ path: agentPath })}`)
  return unwrap(response)
}

/**
 * 获取文件内容
 * @param {string} agentSlug - agent slug
 * @param {string} path - 文件路径（相对于 agent 工作区根）
 * @returns {Promise<Response>} - 返回原始 Response（blob 类型）
 */
export const getViewerFileContent = (agentSlug, path) => {
  const agentPath = buildAgentPath(agentSlug, path)
  return apiGet(`/api/workspace/file?${buildQuery({ path: agentPath })}`, {}, true, 'blob')
}

/**
 * 下载文件
 * @param {string} agentSlug - agent slug
 * @param {string} path - 文件路径（相对于 agent 工作区根）
 * @returns {Promise<Response>} - 返回原始 Response（blob 类型）
 */
export const downloadViewerFile = (agentSlug, path) => {
  const agentPath = buildAgentPath(agentSlug, path)
  return apiGet(`/api/workspace/download?${buildQuery({ path: agentPath })}`, {}, true, 'blob')
}

/**
 * 删除文件或目录
 * @param {string} agentSlug - agent slug
 * @param {string} path - 路径（相对于 agent 工作区根）
 * @returns {Promise}
 */
export const deleteViewerFile = async (agentSlug, path) => {
  const agentPath = buildAgentPath(agentSlug, path)
  const response = await apiDelete(`/api/workspace/file?${buildQuery({ path: agentPath })}`)
  return unwrap(response)
}

/**
 * 创建目录
 * @param {string} agentSlug - agent slug
 * @param {string} parentPath - 父路径（相对于 agent 工作区根）
 * @param {string} name - 目录名
 * @returns {Promise}
 */
export const createViewerDirectory = async (agentSlug, parentPath, name) => {
  const agentParentPath = buildAgentPath(agentSlug, parentPath)
  const response = await apiPost('/api/workspace/directory', {
    parent_path: agentParentPath,
    name
  })
  return unwrap(response)
}

/**
 * 上传文件
 * @param {string} agentSlug - agent slug
 * @param {string} parentPath - 父路径（相对于 agent 工作区根）
 * @param {Array} files - 文件列表
 * @returns {Promise}
 */
export const uploadViewerFiles = async (agentSlug, parentPath, files) => {
  const agentParentPath = buildAgentPath(agentSlug, parentPath)
  const formData = new FormData()
  formData.append('parent_path', agentParentPath)
  files.forEach((file) => formData.append('files', file))
  const response = await apiPost('/api/workspace/upload', formData)
  return unwrap(response)
}