/**
 * Mock 数据模块 - 完整版
 * 用于前端开发时模拟后端 API 响应
 */

if (typeof window !== 'undefined') {
  window.__USE_MOCK__ = true
}
export const USE_MOCK = true
export const MOCK_DELAY = 200
export const mockToken = 'mock-jwt-token-eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9'

export const mockResponse = (data, delay = MOCK_DELAY) => {
  return new Promise((resolve) => {
    setTimeout(() => resolve(JSON.parse(JSON.stringify(data))), delay)
  })
}

// ==================== 智能体 ====================
export const mockAgents = [
  {
    id: 'agent-001', agent_id: 'agent-001', slug: 'assistant', name: '通用助手',
    description: '一个通用的 AI 助手', avatar: '/assets/defaults/agent.png',
    model: 'gpt-4', is_builtin: true, is_subagent: false, capabilities: ['files', 'file_upload'],
    configurable_items: {
      knowledge: { kind: 'knowledge', default: null, options: [] },
      skills: { kind: 'skills', default: null, options: [] }
    },
    config_json: { context: { model: 'gpt-4' } },
    created_at: '2024-01-15T10:30:00Z', updated_at: '2024-07-20T14:20:00Z'
  },
  {
    id: 'agent-002', agent_id: 'agent-002', slug: 'coder', name: '代码助手',
    description: '专注于编程和技术问题', avatar: '/assets/defaults/agent.png',
    model: 'gpt-4-turbo', is_builtin: false, is_subagent: false, capabilities: ['files', 'file_upload'],
    configurable_items: {
      knowledge: { kind: 'knowledge', default: null, options: [] },
      skills: { kind: 'skills', default: null, options: [] }
    },
    config_json: { context: { model: 'gpt-4-turbo' } },
    created_at: '2024-02-20T08:15:00Z', updated_at: '2024-07-18T11:45:00Z'
  }
]

export const mockAgentBackends = [
  { id: 'openai', name: 'OpenAI', models: ['gpt-4', 'gpt-4-turbo', 'gpt-3.5-turbo'] },
  { id: 'anthropic', name: 'Anthropic', models: ['claude-3-opus', 'claude-3-sonnet'] }
]

// ==================== 对话 ====================
export const mockThreads = [
  { id: 'thread-001', agent_id: 'agent-001', title: 'Python 学习咨询', created_at: '2024-07-27T10:00:00Z', updated_at: '2024-07-27T10:30:00Z', is_pinned: true, message_count: 12 },
  { id: 'thread-002', agent_id: 'agent-002', title: 'Vue 3 组件优化', created_at: '2024-07-26T15:20:00Z', updated_at: '2024-07-26T16:45:00Z', is_pinned: false, message_count: 8 }
]

export const mockThreadHistory = [
  // thread-001 的消息
  {
    id: 'msg-001', thread_id: 'thread-001', type: 'human', role: 'user',
    content: '我想学习 Python，应该从哪里开始？',
    created_at: '2024-07-27T10:00:00Z'
  },
  {
    id: 'msg-002', thread_id: 'thread-001', type: 'ai', role: 'assistant',
    content: '## Python 学习路线\n\n学习 Python 是一个很好的选择！以下是建议的学习路径：\n\n### 1. 基础语法\n- 变量和数据类型\n- 条件语句和循环\n- 函数定义\n\n### 2. 进阶内容\n- 面向对象编程\n- 模块和包\n- 异常处理\n\n### 3. 实践项目\n```python\n# 第一个程序\nprint("Hello, World!")\n```\n\n建议每天练习 30 分钟，坚持 3 个月就能掌握基础。',
    status: 'finished',
    model: 'gpt-4',
    tokens_used: 256,
    created_at: '2024-07-27T10:00:15Z'
  },
  {
    id: 'msg-003', thread_id: 'thread-001', type: 'human', role: 'user',
    content: '数据分析需要学习哪些库？',
    created_at: '2024-07-27T10:05:00Z'
  },
  {
    id: 'msg-004', thread_id: 'thread-001', type: 'ai', role: 'assistant',
    content: '## Python 数据分析库\n\n### 核心三件套\n\n| 库 | 用途 |\n|---|------|\n| **NumPy** | 数值计算 |\n| **Pandas** | 数据处理 |\n| **Matplotlib** | 数据可视化 |\n\n### 进阶库\n- **Seaborn** - 统计可视化\n- **Scikit-learn** - 机器学习\n- **Plotly** - 交互式图表\n\n建议先从 Pandas 开始学习，它是数据分析的核心工具。',
    status: 'finished',
    model: 'gpt-4',
    tokens_used: 198,
    isLast: true,
    created_at: '2024-07-27T10:05:15Z'
  },
  // thread-002 的消息
  {
    id: 'msg-010', thread_id: 'thread-002', type: 'human', role: 'user',
    content: 'Vue 3 的 Composition API 怎么用？',
    created_at: '2024-07-26T15:20:00Z'
  },
  {
    id: 'msg-011', thread_id: 'thread-002', type: 'ai', role: 'assistant',
    content: '## Vue 3 Composition API\n\n Composition API 是 Vue 3 的核心特性之一。\n\n### 基本用法\n```vue\n<script setup>\nimport { ref, computed } from \'vue\'\n\nconst count = ref(0)\nconst double = computed(() => count.value * 2)\n\nfunction increment() {\n  count.value++\n}\n</script>\n```\n\n### 常用 API\n- `ref()` - 响应式引用\n- `computed()` - 计算属性\n- `watch()` - 侦听器\n- `onMounted()` - 生命周期钩子',
    status: 'finished',
    model: 'gpt-4-turbo',
    tokens_used: 180,
    isLast: true,
    created_at: '2024-07-26T15:20:15Z'
  }
]

export const mockChatResponse = { response: '这是一个模拟的 AI 响应。', tokens_used: 156, model: 'gpt-4', finish_reason: 'stop' }

// ==================== 系统 ====================
export const mockHealth = { status: 'ok', version: '1.0.0', uptime: 86400 }
export const mockInfo = { success: true, data: { name: 'Qavor AI', version: '1.0.0', description: '智能 AI 助手平台' } }
export const mockConfig = { oidc_enabled: false, max_upload_size: 104857600, chat_model: 'gpt-4' }
export const mockConfigOptions = [
  { key: 'oidc_enabled', value: false, type: 'boolean', description: '启用 OIDC 登录' },
  { key: 'chat_model', value: 'gpt-4', type: 'string', description: '默认聊天模型' }
]
export const mockLogs = [
  { timestamp: '2024-07-27T10:00:00Z', level: 'info', message: '系统启动' },
  { timestamp: '2024-07-27T10:01:00Z', level: 'info', message: '数据库连接成功' }
]

// ==================== MCP ====================
export const mockMcpServers = [
  { id: 'mcp-001', name: '文件系统', type: 'filesystem', status: 'connected', enabled: true },
  { id: 'mcp-002', name: '数据库', type: 'database', status: 'connected', enabled: true }
]
export const mockMcpTools = [
  { name: 'read_file', description: '读取文件', server: '文件系统' },
  { name: 'write_file', description: '写入文件', server: '文件系统' }
]

// ==================== 知识库 ====================
export const mockDatabases = [
  { id: 'kb-001', name: '技术文档', description: '公司技术栈相关文档', document_count: 45, created_at: '2024-01-20T10:00:00Z' },
  { id: 'kb-002', name: '产品手册', description: '产品使用说明', document_count: 32, created_at: '2024-02-15T09:00:00Z' }
]
export const mockAccessibleDatabases = [
  { id: 'kb-001', name: '技术文档' },
  { id: 'kb-002', name: '产品手册' }
]
export const mockDocuments = [
  { id: 'doc-001', name: 'API文档.md', type: 'markdown', status: 'indexed', chunk_count: 15, size: 12560, created_at: '2024-07-20T10:00:00Z', updated_at: '2024-07-25T14:30:00Z' },
  { id: 'doc-002', name: '用户手册.pdf', type: 'pdf', status: 'indexed', chunk_count: 28, size: 2450000, created_at: '2024-07-21T10:00:00Z', updated_at: '2024-07-21T10:00:00Z' },
  { id: 'doc-003', name: '技术白皮书.docx', type: 'word', status: 'indexed', chunk_count: 45, size: 1890000, created_at: '2024-07-22T09:00:00Z', updated_at: '2024-07-26T11:00:00Z' },
  { id: 'doc-004', name: '数据分析报告.xlsx', type: 'excel', status: 'indexed', chunk_count: 12, size: 856000, created_at: '2024-07-23T16:00:00Z', updated_at: '2024-07-23T16:00:00Z' },
  { id: 'doc-005', name: '系统架构设计.md', type: 'markdown', status: 'indexed', chunk_count: 32, size: 28900, created_at: '2024-07-24T13:00:00Z', updated_at: '2024-07-27T08:00:00Z' },
  { id: 'doc-006', name: '产品需求文档.pdf', type: 'pdf', status: 'indexing', chunk_count: 0, size: 3200000, created_at: '2024-07-27T10:00:00Z', updated_at: '2024-07-27T10:00:00Z' },
  { id: 'doc-007', name: '开发规范.md', type: 'markdown', status: 'indexed', chunk_count: 8, size: 5600, created_at: '2024-07-25T11:00:00Z', updated_at: '2024-07-25T11:00:00Z' },
  { id: 'doc-008', name: '测试用例文档.docx', type: 'word', status: 'indexed', chunk_count: 25, size: 980000, created_at: '2024-07-26T14:00:00Z', updated_at: '2024-07-26T16:30:00Z' }
]
export const mockKnowledgeTypes = [
  { id: 'document', name: '文档知识库', description: '支持PDF、Word等文档' }
]
export const mockChunkPresets = [
  { id: 'default', name: '默认分块', description: '按段落分块', chunk_size: 500 }
]
export const mockKnowledgeStats = { total_databases: 2, total_documents: 77, total_chunks: 43 }

// ==================== 技能 ====================
export const mockSkills = [
  { id: 'skill-001', slug: 'web-search', name: '网页搜索', description: '搜索互联网', enabled: true, version: '1.0.0' },
  { id: 'skill-002', slug: 'code-runner', name: '代码执行', description: '执行代码', enabled: true, version: '1.2.0' }
]
export const mockAccessibleSkills = [
  { id: 'skill-001', slug: 'web-search', name: '网页搜索' },
  { id: 'skill-002', slug: 'code-runner', name: '代码执行' }
]
export const mockBuiltinSkills = [
  { id: 'builtin-001', slug: 'calculator', name: '计算器', enabled: true }
]

// ==================== 用户 ====================
export const mockCurrentUser = {
  id: 1, uid: 'user-001', username: 'zhangsan', display_name: '张三',
  role: 'superadmin', avatar: null,
  department_id: null, department_name: null, phone_number: null
}
export const mockUsers = [
  mockCurrentUser,
  { id: 2, uid: 'user-002', username: 'lisi', display_name: '李四', role: 'user' }
]
export const mockUserConfig = { theme: 'light', language: 'zh-CN' }
export const mockAgentEnv = { env: {} }
export const mockApiKeys = [
  { id: 'key-001', name: '默认密钥', prefix: 'sk-xxxx', created_at: '2024-07-20T10:00:00Z' }
]
export const mockAccessOptions = [
  { id: 1, username: 'zhangsan', role: 'admin' },
  { id: 2, username: 'lisi', role: 'user' }
]

// ==================== Dashboard ====================
export const mockDashboardStats = { total_conversations: 1256, total_messages: 8934, active_users: 89, total_agents: 12 }
export const mockDashboardConversations = [
  { id: 'thread-001', uid: 'user-001', user_name: '张三', agent_name: '通用助手', title: 'Python 学习', message_count: 12, status: 'completed', created_at: '2024-07-27T10:00:00Z' },
  { id: 'thread-002', uid: 'user-002', user_name: '李四', agent_name: '数据分析助手', title: '销售数据报表分析', message_count: 8, status: 'completed', created_at: '2024-07-27T09:15:00Z' },
  { id: 'thread-003', uid: 'user-003', user_name: '王五', agent_name: '代码助手', title: 'React 组件性能优化', message_count: 5, status: 'active', created_at: '2024-07-27T08:30:00Z' },
  { id: 'thread-004', uid: 'user-001', user_name: '张三', agent_name: '通用助手', title: 'SQL 查询优化', message_count: 20, status: 'completed', created_at: '2024-07-26T16:00:00Z' },
  { id: 'thread-005', uid: 'user-004', user_name: '赵六', agent_name: '文档助手', title: 'API 文档生成', message_count: 15, status: 'completed', created_at: '2024-07-26T14:00:00Z' },
  { id: 'thread-006', uid: 'user-005', user_name: '钱七', agent_name: '数据分析助手', title: '用户行为分析', message_count: 10, status: 'failed', created_at: '2024-07-26T11:00:00Z' }
]
export const mockDashboardFeedbacks = [
  { id: 'fb-001', rating: 'like', user_name: '张三', agent_name: '通用助手', comment: '回答很有帮助', created_at: '2024-07-27T10:30:00Z' },
  { id: 'fb-002', rating: 'dislike', user_name: '李四', agent_name: '数据分析助手', comment: '结果不够准确', created_at: '2024-07-27T09:45:00Z' },
  { id: 'fb-003', rating: 'like', user_name: '王五', agent_name: '代码助手', comment: '解决了性能问题', created_at: '2024-07-26T16:30:00Z' }
]
export const mockUserStats = { 
  daily_active_users: [
    { date: '7-21', active_users: 45 },
    { date: '7-22', active_users: 52 },
    { date: '7-23', active_users: 48 },
    { date: '7-24', active_users: 61 },
    { date: '7-25', active_users: 55 },
    { date: '7-26', active_users: 68 },
    { date: '7-27', active_users: 58 }
  ],
  total_users: 150,
  weekly_active_users: [65, 72, 58, 89, 76, 92, 58],
  weekly_labels: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'],
  new_users_today: 12,
  new_users_this_week: 45
}
export const mockToolStats = { 
  total_calls: 3456, 
  success_rate: 0.95,
  tool_calls_by_name: [
    { name: 'search', count: 1250, success_rate: 0.98 },
    { name: 'calculator', count: 890, success_rate: 0.99 },
    { name: 'web_scrape', count: 670, success_rate: 0.92 },
    { name: 'file_read', count: 450, success_rate: 0.96 },
    { name: 'database_query', count: 196, success_rate: 0.88 }
  ],
  weekly_tool_calls: [420, 480, 390, 520, 450, 680, 516]
}
export const mockAgentStats = { 
  total_agents: 12, 
  active_agents: 8,
  agent_usage: [
    { name: '通用助手', usage_count: 560, avg_response_time: 3.2 },
    { name: '数据分析助手', usage_count: 320, avg_response_time: 4.5 },
    { name: '代码助手', usage_count: 280, avg_response_time: 3.8 },
    { name: '文档助手', usage_count: 180, avg_response_time: 5.1 },
    { name: '智能客服', usage_count: 120, avg_response_time: 2.8 }
  ],
  weekly_agent_calls: [200, 240, 180, 260, 220, 320, 260]
}
export const mockKnowledgeStatsDash = { 
  total_documents: 234, 
  total_size: '2.5 GB',
  document_types: [
    { type: 'PDF', count: 89, size: '1.2 GB' },
    { type: 'Markdown', count: 67, size: '128 MB' },
    { type: 'Word', count: 45, size: '560 MB' },
    { type: 'Excel', count: 23, size: '340 MB' },
    { type: '其他', count: 10, size: '272 MB' }
  ],
  weekly_document_additions: [12, 8, 15, 6, 20, 18, 12],
  top_knowledge_bases: [
    { name: '技术文档库', document_count: 89, query_count: 2340 },
    { name: '产品手册', document_count: 56, query_count: 1890 },
    { name: 'API 文档', document_count: 45, query_count: 1560 },
    { name: '知识库', document_count: 44, query_count: 890 }
  ]
}
export const mockTimeseries = [
  { timestamp: '2024-07-20', value: 120 },
  { timestamp: '2024-07-21', value: 145 },
  { timestamp: '2024-07-22', value: 98 },
  { timestamp: '2024-07-23', value: 167 },
  { timestamp: '2024-07-24', value: 134 },
  { timestamp: '2024-07-25', value: 189 },
  { timestamp: '2024-07-26', value: 156 },
  { timestamp: '2024-07-27', value: 178 }
]

// ==================== 图谱 ====================
export const mockGraphs = [
  { id: 'graph-001', name: '技术知识图谱', kb_id: 'kb-001', node_count: 156, edge_count: 234 }
]
export const mockGraphSubgraph = {
  nodes: [{ id: 'node-1', label: '概念A', type: 'concept' }, { id: 'node-2', label: '概念B', type: 'concept' }],
  edges: [{ source: 'node-1', target: 'node-2', label: '关联' }]
}
export const mockGraphStats = { node_count: 156, edge_count: 234, label_counts: { concept: 89, entity: 67 } }
export const mockGraphLabels = ['concept', 'entity', 'relation']

// ==================== 部门 ====================
export const mockDepartments = [
  { id: 1, name: '技术部', description: '负责技术研发', member_count: 15 },
  { id: 2, name: '产品部', description: '负责产品设计', member_count: 8 }
]

// ==================== 工具 ====================
export const mockTools = [
  { id: 'tool-001', name: 'search', display_name: '网页搜索', category: 'general', enabled: true },
  { id: 'tool-002', name: 'calculator', display_name: '计算器', category: 'general', enabled: true }
]
export const mockToolOptions = [
  { value: 'search', label: '网页搜索' },
  { value: 'calculator', label: '计算器' }
]

// ==================== 工作区 ====================
export const mockWorkspaceTree = [
  { name: 'user-data', type: 'folder', path: '/user-data', is_dir: true, children: [
    { name: 'documents', type: 'folder', path: '/user-data/documents', is_dir: true, children: [
      { name: '技术文档', type: 'folder', path: '/user-data/documents/技术文档', is_dir: true, children: [
        { name: 'API文档.md', type: 'file', path: '/user-data/documents/技术文档/API文档.md', size: 12560, modified_at: '2024-07-25T14:30:00Z' },
        { name: '系统架构设计.md', type: 'file', path: '/user-data/documents/技术文档/系统架构设计.md', size: 28900, modified_at: '2024-07-27T08:00:00Z' },
        { name: '开发规范.md', type: 'file', path: '/user-data/documents/技术文档/开发规范.md', size: 5600, modified_at: '2024-07-25T11:00:00Z' }
      ]},
      { name: '产品文档', type: 'folder', path: '/user-data/documents/产品文档', is_dir: true, children: [
        { name: '用户手册.pdf', type: 'file', path: '/user-data/documents/产品文档/用户手册.pdf', size: 2450000, modified_at: '2024-07-21T10:00:00Z' },
        { name: '技术白皮书.docx', type: 'file', path: '/user-data/documents/产品文档/技术白皮书.docx', size: 1890000, modified_at: '2024-07-26T11:00:00Z' },
        { name: '产品需求文档.pdf', type: 'file', path: '/user-data/documents/产品文档/产品需求文档.pdf', size: 3200000, modified_at: '2024-07-27T10:00:00Z' }
      ]},
      { name: '数据分析', type: 'folder', path: '/user-data/documents/数据分析', is_dir: true, children: [
        { name: '数据分析报告.xlsx', type: 'file', path: '/user-data/documents/数据分析/数据分析报告.xlsx', size: 856000, modified_at: '2024-07-23T16:00:00Z' },
        { name: '测试用例文档.docx', type: 'file', path: '/user-data/documents/数据分析/测试用例文档.docx', size: 980000, modified_at: '2024-07-26T16:30:00Z' }
      ]},
      { name: 'readme.md', type: 'file', path: '/user-data/documents/readme.md', size: 1024, modified_at: '2024-07-20T09:00:00Z' }
    ]},
    { name: 'config', type: 'folder', path: '/user-data/config', is_dir: true, children: [
      { name: 'config.json', type: 'file', path: '/user-data/config/config.json', size: 2560, modified_at: '2024-07-26T10:00:00Z' },
      { name: 'settings.yaml', type: 'file', path: '/user-data/config/settings.yaml', size: 1536, modified_at: '2024-07-25T15:00:00Z' }
    ]},
    { name: 'scripts', type: 'folder', path: '/user-data/scripts', is_dir: true, children: [
      { name: 'import_data.py', type: 'file', path: '/user-data/scripts/import_data.py', size: 4096, modified_at: '2024-07-24T14:00:00Z' },
      { name: 'backup.sh', type: 'file', path: '/user-data/scripts/backup.sh', size: 1024, modified_at: '2024-07-22T11:00:00Z' }
    ]},
    { name: 'logs', type: 'folder', path: '/user-data/logs', is_dir: true, children: [
      { name: 'app.log', type: 'file', path: '/user-data/logs/app.log', size: 1048576, modified_at: '2024-07-27T12:00:00Z' },
      { name: 'error.log', type: 'file', path: '/user-data/logs/error.log', size: 524288, modified_at: '2024-07-27T11:30:00Z' }
    ]}
  ]}
]

// ==================== 模型提供商 ====================
export const mockModelProviders = [
  { id: 'openai', name: 'OpenAI', type: 'openai', enabled: true },
  { id: 'anthropic', name: 'Anthropic', type: 'anthropic', enabled: true }
]
export const mockModels = [
  { id: 'gpt-4', name: 'GPT-4', provider: 'openai', type: 'chat' },
  { id: 'gpt-4-turbo', name: 'GPT-4 Turbo', provider: 'openai', type: 'chat' }
]
