import { apiDelete, apiGet, apiPost, apiPut } from './base'

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

export const getWorkspaceTree = async (path = '/', recursive = false, filesOnly = false) => {
  const query = buildQuery({ path, recursive, files_only: filesOnly })
  const response = await apiGet(`/api/workspace/tree?${query}`)
  return unwrap(response)
}

export const getWorkspaceFileContent = (path) => {
  const query = buildQuery({ path })
  return apiGet(`/api/workspace/file?${query}`, {}, true, 'blob')
}

export const saveWorkspaceFileContent = async (path, content) => {
  const response = await apiPut('/api/workspace/file', { path, content })
  return unwrap(response)
}

export const deleteWorkspacePath = async (path) => {
  const query = buildQuery({ path })
  const response = await apiDelete(`/api/workspace/file?${query}`)
  return unwrap(response)
}

export const createWorkspaceDirectory = async (parentPath, name) => {
  const response = await apiPost('/api/workspace/directory', {
    parent_path: parentPath,
    name
  })
  return unwrap(response)
}

export const uploadWorkspaceFiles = async (parentPath, files) => {
  const formData = new FormData()
  formData.append('parent_path', parentPath)
  files.forEach((file) => formData.append('files', file))
  const response = await apiPost('/api/workspace/upload', formData)
  return unwrap(response)
}
