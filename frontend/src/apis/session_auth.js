export async function loginWithPassword(fetchImpl, credentials) {
  const response = await fetchImpl('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      username: credentials.username,
      password: credentials.password
    })
  })

  const payload = await response.json()
  if (!response.ok || payload.code !== 0) {
    throw new Error(payload.message || '登录失败')
  }
  if (!payload.data?.token) {
    throw new Error('登录响应缺少令牌')
  }
  return payload.data.token
}

export async function refreshAccessToken(fetchImpl) {
	const response = await fetchImpl('/api/v1/auth/refresh', {
		method: 'POST',
		credentials: 'same-origin'
	})
	const payload = await response.json()
	if (!response.ok || payload.code !== 0 || !payload.data?.token) {
		throw new Error(payload.message || '刷新登录状态失败')
	}
	return payload.data.token
}

export async function logoutWithToken(fetchImpl, token) {
	if (!token) return

	const response = await fetchImpl('/api/v1/auth/logout', {
		method: 'POST',
		headers: { Authorization: `Bearer ${token}` },
		credentials: 'same-origin'
	})
	if (!response.ok) {
		throw new Error('登出请求失败')
	}
}
