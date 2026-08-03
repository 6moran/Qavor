/**
 * 用户相关 Mock 数据
 */

export const mockCurrentUser = {
  id: 1,
  uid: 'user-001',
  username: 'zhangsan',
  display_name: '张三',
  role: 'superadmin',
  avatar: null,
  department_id: null,
  department_name: null,
  phone_number: '13800138000',
  status: 1,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-07-27T00:00:00Z'
}

export const mockUsers = [
  mockCurrentUser,
  {
    id: 2,
    uid: 'user-002',
    username: 'lisi',
    display_name: '李四',
    role: 'admin',
    avatar: null,
    department_id: null,
    department_name: null,
    phone_number: null,
    status: 1,
    created_at: '2024-02-01T00:00:00Z',
    updated_at: '2024-07-20T00:00:00Z'
  },
  {
    id: 3,
    uid: 'user-003',
    username: 'wangwu',
    display_name: '王五',
    role: 'user',
    avatar: null,
    department_id: 1,
    department_name: '技术部',
    phone_number: null,
    status: 1,
    created_at: '2024-03-01T00:00:00Z',
    updated_at: '2024-07-15T00:00:00Z'
  }
]

export const mockToken = 'mock-jwt-token-eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9'

export const mockUserConfig = {
  theme: 'light',
  language: 'zh-CN'
}
