import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { loginWithPassword, logoutWithToken } from '@/apis/session_auth'
import { useAgentStore } from './agent'

export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('user_token') || '')
  const username = ref(localStorage.getItem('admin_username') || '')

  // 兼容尚未完成单实例化的旧组件；这些值不再代表数据库用户。
  const userId = computed(() => (token.value ? 1 : null))
  const uid = ref('')
  const phoneNumber = ref('')
  const avatar = ref('')
  const userRole = computed(() => (token.value ? 'admin' : ''))
  const departmentId = ref(null)
  const departmentName = ref('')

  const isLoggedIn = computed(() => Boolean(token.value))
  const isAdmin = computed(() => Boolean(token.value))
  const isSuperAdmin = computed(() => Boolean(token.value))

  async function login(credentials) {
    const nextToken = await loginWithPassword(fetch, credentials)
    token.value = nextToken
    username.value = credentials.username
    localStorage.setItem('user_token', nextToken)
    localStorage.setItem('admin_username', credentials.username)
    return true
  }

  function logout() {
		const currentToken = token.value
		if (currentToken) {
			void logoutWithToken(fetch, currentToken).catch((error) => {
				console.warn('后端登出失败，令牌将自然过期:', error)
			})
		}
		token.value = ''
    username.value = ''
    localStorage.removeItem('user_token')
    localStorage.removeItem('admin_username')
    useAgentStore().reset()
  }

  function getAuthHeaders() {
    return token.value ? { Authorization: `Bearer ${token.value}` } : {}
  }

  async function getCurrentUser() {
    return { username: username.value, role: 'admin' }
  }

  return {
    token,
    userId,
    username,
    uid,
    phoneNumber,
    avatar,
    userRole,
    departmentId,
    departmentName,
    isLoggedIn,
    isAdmin,
    isSuperAdmin,
    login,
    logout,
    getAuthHeaders,
    getCurrentUser
  }
})

export const checkAdminPermission = () => {
  const userStore = useUserStore()
  if (!userStore.isAdmin) throw new Error('需要管理员权限')
  return true
}

export const checkSuperAdminPermission = () => useUserStore().isSuperAdmin
