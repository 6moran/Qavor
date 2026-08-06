import { apiDelete, apiGet, apiPost } from './base'

const buildQuery = (params) => {
  const query = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value))
    }
  })
  return query.toString()
}

const buildViewerQuery = (threadId, path) => {
  return buildQuery({
    thread_id: threadId,
    path
  })
}

/**
 * 获取文件系统树
 * @param {string} threadId - 线程ID
 * @param {string} path - 路径
 * @returns {Promise}（后端未实现，返回空数组）
 */
export const getViewerFileSystemTree = (threadId, path = '/') => {
  console.warn('getViewerFileSystemTree: 后端未实现此接口')
  return Promise.resolve({ data: [] })
}

/**
 * 获取文件内容
 * @param {string} threadId - 线程ID
 * @param {string} path - 文件路径
 * @returns {Promise}（后端未实现，返回空）
 */
export const getViewerFileContent = (threadId, path) => {
  console.warn('getViewerFileContent: 后端未实现此接口')
  return Promise.resolve(null)
}

/**
 * 下载文件
 * @param {string} threadId - 线程ID
 * @param {string} path - 文件路径
 * @returns {Promise}（后端未实现，返回空）
 */
export const downloadViewerFile = (threadId, path) => {
  console.warn('downloadViewerFile: 后端未实现此接口')
  return Promise.resolve(null)
}

/**
 * 删除文件
 * @param {string} threadId - 线程ID
 * @param {string} path - 文件路径
 * @returns {Promise}（后端未实现，返回成功）
 */
export const deleteViewerFile = (threadId, path) => {
  console.warn('deleteViewerFile: 后端未实现此接口')
  return Promise.resolve({ success: true })
}

/**
 * 创建目录
 * @param {string} threadId - 线程ID
 * @param {string} parentPath - 父路径
 * @param {string} name - 目录名
 * @returns {Promise}（后端未实现，返回成功）
 */
export const createViewerDirectory = (threadId, parentPath, name) => {
  console.warn('createViewerDirectory: 后端未实现此接口')
  return Promise.resolve({ success: true })
}

/**
 * 上传文件
 * @param {string} threadId - 线程ID
 * @param {string} parentPath - 父路径
 * @param {Array} files - 文件列表
 * @returns {Promise}（后端未实现，返回成功）
 */
export const uploadViewerFiles = (threadId, parentPath, files) => {
  console.warn('uploadViewerFiles: 后端未实现此接口')
  return Promise.resolve({ success: true })
}
