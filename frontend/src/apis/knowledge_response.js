/**
 * 解包 Go 后端统一响应。
 *
 * 知识库接口可能使用 HTTP 200 表示请求已到达服务端，再通过 code 表示业务结果，
 * 因此不能只依赖 fetch 的 response.ok 判断成功。
 */
export function unwrapKnowledgeResponse(response) {
  if (!response || typeof response !== 'object' || typeof response.code !== 'number') {
    throw new Error('知识库接口返回格式错误')
  }

  if (response.code !== 0) {
    const error = new Error(response.message || '知识库接口请求失败')
    error.code = response.code
    error.data = response.data
    throw error
  }

  return response.data
}

/**
 * 将后端知识库分页数据转换成现有 Store 使用的数据结构。
 */
export function adaptKnowledgeBaseList(data) {
  return {
    total: Number(data?.total) || 0,
    databases: Array.isArray(data?.items) ? data.items : []
  }
}

/**
 * 为后端文件分页结果补齐现有文件浏览器依赖的分页字段。
 */
export function adaptKnowledgeFileList(data, page = 1, pageSize = 100) {
  const total = Number(data?.total) || 0
  const items = Array.isArray(data?.items) ? data.items : []

  return {
    total,
    items,
    page,
    page_size: pageSize,
    has_more: page * pageSize < total
  }
}

