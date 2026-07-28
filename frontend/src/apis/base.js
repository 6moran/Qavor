import { useUserStore, checkAdminPermission, checkSuperAdminPermission } from '@/stores/user'
import { message } from 'ant-design-vue'
import {
  USE_MOCK, mockResponse,
  // 智能体
  mockAgents, mockAgentBackends, mockThreads, mockThreadHistory, mockChatResponse,
  // 系统
  mockHealth, mockInfo, mockConfig, mockConfigOptions, mockLogs,
  // MCP
  mockMcpServers, mockMcpTools,
  // 知识库
  mockDatabases, mockAccessibleDatabases, mockDocuments, mockKnowledgeTypes, mockChunkPresets, mockKnowledgeStats,
  // 技能
  mockSkills, mockAccessibleSkills, mockBuiltinSkills,
  // 用户
  mockCurrentUser, mockUsers, mockUserConfig, mockAgentEnv, mockApiKeys, mockAccessOptions,
  // Dashboard
  mockDashboardStats, mockDashboardConversations, mockDashboardFeedbacks, mockUserStats, mockToolStats, mockAgentStats, mockKnowledgeStatsDash, mockTimeseries,
  // 图谱
  mockGraphs, mockGraphSubgraph, mockGraphStats, mockGraphLabels,
  // 部门
  mockDepartments,
  // 工具
  mockTools, mockToolOptions,
  // 工作区
  mockWorkspaceTree,
  // 任务
  mockTasks,
  // 模型
  mockModelProviders, mockModels
} from '@/mock'

/**
 * 基础API请求封装
 * 提供统一的请求方法，自动处理认证头和错误
 */

/**
 * 根据 URL 和方法获取 mock 数据
 */
function getMockData(url, method, options) {
  // ==================== 系统 ====================
  if (url === '/api/system/health') return mockResponse(mockHealth)
  if (url === '/api/system/info') return mockResponse(mockInfo)
  if (url === '/api/system/config' && method === 'GET') return mockResponse(mockConfig)
  if (url === '/api/system/config' && method === 'POST') return mockResponse({ success: true })
  if (url === '/api/system/config/update') return mockResponse({ success: true })
  if (url === '/api/system/config/options') return mockResponse(mockConfigOptions)
  if (url.startsWith('/api/system/config/options/')) return mockResponse({ success: true })
  if (url.startsWith('/api/system/logs')) return mockResponse(mockLogs)
  if (url.startsWith('/api/system/mcp-servers') && method === 'GET') return mockResponse({ data: mockMcpServers })
  if (url.startsWith('/api/system/mcp-servers') && method === 'POST') return mockResponse({ success: true })
  if (url.startsWith('/api/system/mcp-servers/') && url.endsWith('/tools') && method === 'GET') return mockResponse({ data: mockMcpTools })
  if (url.startsWith('/api/system/mcp-servers/')) return mockResponse({ success: true })
  if (url.startsWith('/api/system/skills') && method === 'GET') return mockResponse({ skills: mockSkills })
  if (url.startsWith('/api/system/skills/builtin')) return mockResponse({ skills: mockBuiltinSkills })
  if (url.startsWith('/api/system/skills/')) return mockResponse({ success: true })
  if (url.startsWith('/api/system/tools')) return mockResponse({ tools: mockTools })
  if (url.startsWith('/api/system/tools/options')) return mockResponse({ options: mockToolOptions })
  if (url.startsWith('/api/system/ocr/options')) return mockResponse({ enabled: true })
  if (url.startsWith('/api/system/ocr/health')) return mockResponse({ status: 'ok' })
  if (url.startsWith('/api/system/model-providers')) return mockResponse({ providers: mockModelProviders })
  if (url.includes('/api/system/model-providers/')) return mockResponse({ success: true })

  // ==================== 认证 ====================
  if (url.startsWith('/api/auth/me')) return mockResponse(mockCurrentUser)
  if (url.startsWith('/api/auth/check-first-run')) return mockResponse({ first_run: false })
  if (url.startsWith('/api/auth/oidc/config')) return mockResponse({ enabled: false })
  if (url.startsWith('/api/auth/oidc/login-url')) return mockResponse({ login_url: '/login' })
  if (url.startsWith('/api/auth/oidc/exchange-code')) return mockResponse({ access_token: 'mock-token', user_id: 1 })
  if (url.startsWith('/api/auth/initialize')) return mockResponse({ access_token: 'mock-token', user_id: 1, username: 'admin', role: 'admin' })
  if (url.startsWith('/api/auth/token')) return mockResponse({ access_token: 'mock-token', user_id: 1, username: 'zhangsan', role: 'admin' })
  if (url.startsWith('/api/auth/users')) return mockResponse({ users: mockUsers })
  if (url.startsWith('/api/auth/users/access-options')) return mockResponse({ options: mockAccessOptions })
  if (url.startsWith('/api/auth/cli/sessions/')) return mockResponse({ status: 'pending', key_name: 'Qavor CLI' })

  // ==================== 用户 ====================
  if (url.startsWith('/api/user/config') && method === 'GET') return mockResponse(mockUserConfig)
  if (url.startsWith('/api/user/config') && method === 'PUT') return mockResponse({ success: true })
  if (url.startsWith('/api/user/agent-env') && method === 'GET') return mockResponse(mockAgentEnv)
  if (url.startsWith('/api/user/agent-env') && method === 'PUT') return mockResponse({ success: true })
  if (url.startsWith('/api/user/apikey') && method === 'GET') return mockResponse({ api_keys: mockApiKeys })
  if (url.startsWith('/api/user/apikey')) return mockResponse({ success: true })
  if (url.startsWith('/api/user/upload-image')) return mockResponse({ url: '/assets/defaults/agent.png' })

  // ==================== 智能体 ====================
  if (url.startsWith('/api/agent/backends')) return mockResponse({ backends: mockAgentBackends })
  if (url.startsWith('/api/agent') && method === 'GET') return mockResponse({ agents: mockAgents })
  if (url.startsWith('/api/agent') && method === 'POST') return mockResponse({ success: true, agent: mockAgents[0] })
  if (url.match(/^\/api\/agent\/[^/]+/) && method === 'GET') {
    const id = url.split('/api/agent/')[1].split('?')[0].split('/')[0]
    return mockResponse(mockAgents.find(a => a.id === id) || mockAgents[0])
  }
  if (url.match(/^\/api\/agent\/[^/]+/) && (method === 'PUT' || method === 'DELETE')) return mockResponse({ success: true })
  if (url.startsWith('/api/agent/runs')) return mockResponse({ run_id: 'run-001', status: 'completed' })
  if (url.startsWith('/api/agent/runs/')) return mockResponse({ status: 'completed' })
  if (url.startsWith('/api/agent/requests/')) return mockResponse({ status: 'completed' })
  if (url.startsWith('/api/agent/thread/')) return mockResponse({ requests: [] })

  // ==================== 聊天 ====================
  if (url.startsWith('/api/chat/call') && method === 'POST') return mockResponse(mockChatResponse)
  if (url.startsWith('/api/chat/threads')) return mockResponse(mockThreads)
  if (url.startsWith('/api/chat/thread') && method === 'POST') return mockResponse({ id: 'thread-new', title: '新对话' })
  if (url.match(/^\/api\/chat\/thread\/[^/]+/) && method === 'PUT') return mockResponse({ success: true })
  if (url.match(/^\/api\/chat\/thread\/[^/]+/) && method === 'DELETE') return mockResponse({ success: true })
  if (url.match(/^\/api\/chat\/thread\/[^/]+\/history/)) {
    const threadId = url.split('/api/chat/thread/')[1].split('/history')[0].split('?')[0]
    return mockResponse(mockThreadHistory.filter(m => m.thread_id === threadId))
  }
  if (url.match(/^\/api\/chat\/thread\/[^/]+\/state/)) return mockResponse({ agent_state: { thread_id: url.split('/').slice(-2)[0].split('?')[0], status: 'idle', token_usage: { prompt: 256, completion: 198, total: 454, llm_input_tokens: 256, llm_output_tokens: 198 }, files: { '/user-data/documents/技术文档/API文档.md': { name: 'API文档.md', type: 'file', size: 12560 }, '/user-data/documents/技术文档/系统架构设计.md': { name: '系统架构设计.md', type: 'file', size: 28900 } }, artifacts: [{ path: '/user-data/scripts/generated_report.py', name: 'generated_report.py', size: 4096, type: 'file' }], todos: [{ id: 'todo-1', content: '完成 API 文档编写', status: 'completed' }, { id: 'todo-2', content: '优化 Vue 组件性能', status: 'in_progress' }, { id: 'todo-3', content: '编写单元测试', status: 'pending' }], subagent_runs: [] } })
  if (url.match(/^\/api\/chat\/thread\/[^/]+\/attachments/)) return mockResponse({ attachments: [] })
  if (url.match(/^\/api\/chat\/thread\/[^/]+\/files/)) return mockResponse({ entries: mockWorkspaceTree })
  if (url.match(/^\/api\/chat\/thread\/[^/]+\/artifacts/)) return mockResponse({ artifacts: [] })
  if (url.match(/^\/api\/chat\/message\/[^/]+\/feedback/)) return mockResponse({ feedback: null })
  if (url.startsWith('/api/chat/image/upload')) return mockResponse({ url: '/tmp/image.png' })
  if (url.startsWith('/api/chat/attachments/tmp')) return mockResponse({ id: 'att-001' })
  if (url.startsWith('/api/chat/attachments/tmp/parse')) return mockResponse({ content: '文件内容' })

  // ==================== 知识库 ====================
  if (url.startsWith('/api/knowledge/databases/accessible')) return mockResponse({ databases: mockAccessibleDatabases })
  if (url.startsWith('/api/knowledge/databases') && method === 'GET') return mockResponse({ databases: mockDatabases })
  if (url.startsWith('/api/knowledge/databases') && method === 'POST') return mockResponse({ success: true, database: mockDatabases[0] })
  if (url.match(/^\/api\/knowledge\/databases\/[^/]+/) && method === 'GET') return mockResponse(mockDatabases[0])
  if (url.match(/^\/api\/knowledge\/databases\/[^/]+/) && (method === 'PUT' || method === 'DELETE')) return mockResponse({ success: true })
  if (url.includes('/documents') && method === 'GET') return mockResponse({ documents: mockDocuments })
  if (url.includes('/documents') && method === 'POST') return mockResponse({ success: true })
  if (url.includes('/folders') && method === 'POST') return mockResponse({ success: true })
  if (url.includes('/query') && method === 'POST') return mockResponse({ results: [], answer: '模拟查询结果' })
  if (url.includes('/mindmap')) return mockResponse({ mindmap: { nodes: [], edges: [] } })
  if (url.includes('/graph-build')) return mockResponse({ status: 'idle' })
  if (url.startsWith('/api/knowledge/types')) return mockResponse({ types: mockKnowledgeTypes })
  if (url.startsWith('/api/knowledge/chunk-presets')) return mockResponse({ presets: mockChunkPresets })
  if (url.startsWith('/api/knowledge/stats')) return mockResponse(mockKnowledgeStats)
  if (url.includes('/evaluation/')) return mockResponse({ datasets: [], runs: [] })

  // ==================== 技能 ====================
  if (url.startsWith('/api/skills/accessible')) return mockResponse({ skills: mockAccessibleSkills })
  if (url.includes('/skills/remote/list')) return mockResponse({ skills: [] })
  if (url.includes('/skills/remote/search')) return mockResponse({ skills: [] })
  if (url.startsWith('/api/skills/')) return mockResponse({ success: true })

  // ==================== MCP ====================
  if (url.startsWith('/api/system/mcp-servers') && method === 'GET') return mockResponse({ data: mockMcpServers })

  // ==================== Dashboard ====================
  if (url.startsWith('/api/dashboard/stats') && method === 'GET') return mockResponse(mockDashboardStats)
  if (url.startsWith('/api/dashboard/stats/users')) return mockResponse(mockUserStats)
  if (url.startsWith('/api/dashboard/stats/tools')) return mockResponse(mockToolStats)
  if (url.startsWith('/api/dashboard/stats/agents')) return mockResponse(mockAgentStats)
  if (url.startsWith('/api/dashboard/stats/knowledge')) return mockResponse(mockKnowledgeStatsDash)
  if (url.includes('/calls/timeseries')) return mockResponse(mockTimeseries)
  if (url.startsWith('/api/dashboard/conversations')) return mockResponse({ conversations: mockDashboardConversations })
  if (url.startsWith('/api/dashboard/feedbacks')) return mockResponse({ feedbacks: mockDashboardFeedbacks })

  // ==================== 图谱 ====================
  if (url.startsWith('/api/graph/list')) return mockResponse({ graphs: mockGraphs })
  if (url.startsWith('/api/graph/subgraph')) return mockResponse(mockGraphSubgraph)
  if (url.startsWith('/api/graph/stats')) return mockResponse(mockGraphStats)
  if (url.startsWith('/api/graph/labels')) return mockResponse({ labels: mockGraphLabels })

  // ==================== 部门 ====================
  if (url.startsWith('/api/departments') && method === 'GET') return mockResponse({ departments: mockDepartments })
  if (url.startsWith('/api/departments') && method === 'POST') return mockResponse({ success: true })
  if (url.match(/^\/api\/departments\/[^/]+/)) return mockResponse({ success: true })

  // ==================== 工作区 ====================
  if (url.startsWith('/api/workspace/tree')) return mockResponse({ entries: mockWorkspaceTree })
  if (url.startsWith('/api/workspace/file')) return mockResponse({ content: '文件内容' })
  if (url.includes('/workspace/')) return mockResponse({ success: true })

  // ==================== Viewer ====================
  if (url.includes('/viewer/filesystem/tree')) return mockResponse({ entries: mockWorkspaceTree })
  if (url.includes('/viewer/filesystem/')) return mockResponse({ success: true })

  // ==================== 任务 ====================
  if (url.startsWith('/api/tasks')) return mockResponse({ tasks: mockTasks })
  if (url.match(/^\/api\/tasks\/[^/]+/)) return mockResponse({ success: true })

  // ==================== Mention ====================
  if (url.startsWith('/api/mention/search')) return mockResponse([])

  // 未匹配
  return undefined
}

/**
 * 发送API请求的基础函数
 * @param {string} url - API端点
 * @param {Object} options - 请求选项
 * @param {boolean} requiresAuth - 是否需要认证头
 * @param {string} responseType - 响应类型: 'json' | 'text' | 'blob'
 * @returns {Promise} - 请求结果
 */
export async function apiRequest(url, options = {}, requiresAuth = true, responseType = 'json') {
  // Mock 数据拦截
  if (USE_MOCK) {
    const method = options?.method || 'GET'
    const mockResult = getMockData(url, method, options)
    if (mockResult !== undefined) {
      return mockResult
    }
  }

  try {
    const isFormData = options?.body instanceof FormData
    // 默认请求配置
    const requestOptions = {
      ...options,
      headers: {
        ...(!isFormData ? { 'Content-Type': 'application/json' } : {}),
        ...options.headers
      }
    }

    // 如果需要认证，添加认证头
    if (requiresAuth) {
      const userStore = useUserStore()
      if (!userStore.isLoggedIn) {
        throw new Error('用户未登录')
      }

      Object.assign(requestOptions.headers, userStore.getAuthHeaders())
    }

    // 发送请求
    const response = await fetch(url, requestOptions)

    // 处理API返回的错误
    if (!response.ok) {
      // 尝试解析错误信息
      let errorMessage = `请求失败: ${response.status}, ${response.statusText}`
      let errorData = null

      console.log('API请求失败:', {
        url,
        status: response.status,
        statusText: response.statusText,
        headers: Object.fromEntries(response.headers.entries())
      })

      try {
        errorData = await response.json()
        // detail 可能是字符串，也可能是结构化对象（如 { error, message }），后者需取出可读文案，
        // 否则直接拼接会得到 "[object Object]"。
        const detail = errorData.detail
        if (detail && typeof detail === 'object') {
          errorMessage = detail.message || detail.error || errorMessage
        } else {
          errorMessage = detail || errorData.message || errorMessage
        }
        console.log('API错误详情:', errorData)

        // 如果是422错误，打印更详细的信息
        if (response.status === 422) {
          console.error('422验证错误详情:', {
            url,
            requestMethod: requestOptions.method,
            requestHeaders: requestOptions.headers,
            requestBody: requestOptions.body,
            responseData: errorData
          })
        }
      } catch (e) {
        // 如果无法解析JSON，使用默认错误信息
        console.log('无法解析错误响应JSON:', e)
      }

      // 特殊处理401和403错误
      const error = new Error(errorMessage)
      error.response = {
        status: response.status,
        statusText: response.statusText,
        data: errorData
      }

      if (response.status === 401) {
        // 如果是认证失败，可能需要重新登录
        const userStore = useUserStore()

        // 检查是否是token过期（errorMessage 已统一为字符串，避免对对象 detail 调用 includes 抛错）
        const isTokenExpired =
          errorMessage?.includes('令牌已过期') || errorMessage?.includes('token expired')

        message.error(isTokenExpired ? '登录已过期，请重新登录' : '认证失败，请重新登录')

        // 如果用户当前认为自己已登录，则登出
        if (userStore.isLoggedIn) {
          userStore.logout()
        }

        // 使用setTimeout确保消息显示后再跳转
        setTimeout(() => {
          window.location.href = '/login'
        }, 1500)

        throw error
      } else if (response.status === 403) {
        error.message = '没有权限执行此操作'
        throw error
      } else if (response.status === 500) {
        error.message = '服务器内部错误，请使用 docker logs api-dev 查看详细日志'
        throw error
      }

      throw error
    }

    // 根据responseType处理响应
    if (responseType === 'blob') {
      return response
    } else if (responseType === 'json') {
      // 检查Content-Type以确定如何处理响应
      const contentType = response.headers.get('Content-Type')
      if (contentType && contentType.includes('application/json')) {
        return await response.json()
      }
      return await response.text()
    } else if (responseType === 'text') {
      return await response.text()
    } else {
      return response
    }
  } catch (error) {
    if (error.name !== 'AbortError') {
      console.error('API请求错误:', error)
    }
    throw error
  }
}

/**
 * 发送GET请求
 * @param {string} url - API端点
 * @param {Object} options - 请求选项
 * @param {boolean} requiresAuth - 是否需要认证
 * @param {string} responseType - 响应类型: 'json' | 'text' | 'blob'
 * @returns {Promise} - 请求结果
 */
export function apiGet(url, options = {}, requiresAuth = true, responseType = 'json') {
  return apiRequest(url, { method: 'GET', ...options }, requiresAuth, responseType)
}

export function apiAdminGet(url, options = {}, responseType = 'json') {
  checkAdminPermission()
  return apiGet(url, options, true, responseType)
}

export function apiSuperAdminGet(url, options = {}, responseType = 'json') {
  checkSuperAdminPermission()
  return apiGet(url, options, true, responseType)
}

/**
 * 发送POST请求
 * @param {string} url - API端点
 * @param {Object} data - 请求体数据
 * @param {Object} options - 其他请求选项
 * @param {boolean} requiresAuth - 是否需要认证
 * @param {string} responseType - 响应类型: 'json' | 'text' | 'blob'
 * @returns {Promise} - 请求结果
 */
export function apiPost(url, data = {}, options = {}, requiresAuth = true, responseType = 'json') {
  return apiRequest(
    url,
    {
      method: 'POST',
      body: data instanceof FormData ? data : JSON.stringify(data),
      ...options
    },
    requiresAuth,
    responseType
  )
}

export function apiAdminPost(url, data = {}, options = {}, responseType = 'json') {
  checkAdminPermission()
  return apiPost(url, data, options, true, responseType)
}

export function apiSuperAdminPost(url, data = {}, options = {}, responseType = 'json') {
  checkSuperAdminPermission()
  return apiPost(url, data, options, true, responseType)
}

/**
 * 发送PUT请求
 * @param {string} url - API端点
 * @param {Object} data - 请求体数据
 * @param {Object} options - 其他请求选项
 * @param {boolean} requiresAuth - 是否需要认证
 * @param {string} responseType - 响应类型: 'json' | 'text' | 'blob'
 * @returns {Promise} - 请求结果
 */
export function apiPut(url, data = {}, options = {}, requiresAuth = true, responseType = 'json') {
  return apiRequest(
    url,
    {
      method: 'PUT',
      body: data instanceof FormData ? data : JSON.stringify(data),
      ...options
    },
    requiresAuth,
    responseType
  )
}

export function apiAdminPut(url, data = {}, options = {}, responseType = 'json') {
  checkAdminPermission()
  return apiPut(url, data, options, true, responseType)
}

export function apiSuperAdminPut(url, data = {}, options = {}, responseType = 'json') {
  checkSuperAdminPermission()
  return apiPut(url, data, options, true, responseType)
}

/**
 * 发送DELETE请求
 * @param {string} url - API端点
 * @param {Object} options - 请求选项
 * @param {boolean} requiresAuth - 是否需要认证
 * @param {string} responseType - 响应类型: 'json' | 'text' | 'blob'
 * @returns {Promise} - 请求结果
 */
export function apiDelete(url, options = {}, requiresAuth = true, responseType = 'json') {
  return apiRequest(url, { method: 'DELETE', ...options }, requiresAuth, responseType)
}

export function apiAdminDelete(url, options = {}) {
  checkAdminPermission()
  return apiDelete(url, options, true)
}

export function apiSuperAdminDelete(url, options = {}) {
  checkSuperAdminPermission()
  return apiDelete(url, options, true)
}
