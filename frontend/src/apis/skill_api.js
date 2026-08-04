import { apiGet, apiPost, apiPut, apiDelete, apiAdminGet, apiAdminPost } from './base'

const BASE_URL = '/api/v1/system/skills'
const USER_BASE_URL = '/api/v1/skills'

export const listSkills = async () => {
  // 后端响应为 { code, data: { list: [...], total, page, size } }
  const result = await apiGet(BASE_URL)
  if (result?.data?.list) {
    return { success: result.code === 0, data: result.data.list, message: result.message }
  }
  return { success: result?.code === 0, data: result?.data || [], message: result?.message }
}

export const listAccessibleSkills = async () => {
  // 后端响应为 { code, data: { skills: [...], total } }
  const result = await apiGet(`${USER_BASE_URL}/accessible`)
  if (result?.data?.skills) {
    return { success: result.code === 0, data: result.data.skills, message: result.message }
  }
  return { success: result?.code === 0, data: result?.data || [], message: result?.message }
}

export const prepareSkillUpload = async (file) => {
  const formData = new FormData()
  formData.append('file', file)
  // import/prepare 属于系统级路由，在 /system/skills 下
  return apiPost(`${BASE_URL}/import/prepare`, formData)
}

export const listRemoteSkills = async (source) => {
  return apiPost(`${USER_BASE_URL}/remote/list`, { source })
}

export const prepareRemoteSkills = async (payload) => {
  return apiPost(`${USER_BASE_URL}/remote/prepare`, payload)
}

export const getSkillDependencyOptions = async (slug) => {
  const query = slug ? `?slug=${encodeURIComponent(slug)}` : ''
  return apiGet(`${BASE_URL}/dependency-options${query}`)
}

export const listBuiltinSkills = async () => {
  // 后端响应为 { code, data: { skills: [...] } }
  const result = await apiAdminGet(`${BASE_URL}/builtin`)
  if (result?.data?.skills) {
    return { success: result.code === 0, data: result.data.skills, message: result.message }
  }
  return { success: result?.code === 0, data: result?.data || [], message: result?.message }
}

export const syncBuiltinSkills = async () => {
  return apiAdminPost(`${BASE_URL}/builtin/sync`)
}

export const getSkillTree = async (slug) => {
  return apiGet(`${BASE_URL}/${encodeURIComponent(slug)}/tree`)
}

export const getSkillFile = async (slug, path) => {
  return apiGet(`${BASE_URL}/${encodeURIComponent(slug)}/file?path=${encodeURIComponent(path)}`)
}

export const createSkillFile = async (slug, payload) => {
  return apiPost(`${BASE_URL}/${encodeURIComponent(slug)}/file`, payload)
}

export const updateSkillFile = async (slug, payload) => {
  return apiPut(`${BASE_URL}/${encodeURIComponent(slug)}/file`, payload)
}

export const updateSkillDependencies = async (slug, payload) => {
  return apiPut(`${BASE_URL}/${encodeURIComponent(slug)}/dependencies`, payload)
}

export const updateSkillShareConfig = async (slug, shareConfig) => {
  return apiPut(`${BASE_URL}/${encodeURIComponent(slug)}/share-config`, {
    share_config: shareConfig
  })
}

export const updateSkillEnabled = async (slug, enabled) => {
  return apiPut(`${BASE_URL}/${encodeURIComponent(slug)}/enabled`, { enabled })
}

export const deleteSkillFile = async (slug, path) => {
  return apiDelete(`${BASE_URL}/${encodeURIComponent(slug)}/file?path=${encodeURIComponent(path)}`)
}

export const exportSkill = async (slug) => {
  return apiGet(`${BASE_URL}/${encodeURIComponent(slug)}/export`, {}, true, 'blob')
}

export const deleteSkill = async (slug) => {
  return apiDelete(`${BASE_URL}/${encodeURIComponent(slug)}`)
}

export const deleteSkillsBatch = async (slugs) => {
  return apiPost(`${BASE_URL}/delete-batch`, { slugs })
}

export const skillApi = {
  listSkills,
  listAccessibleSkills,
  prepareSkillUpload,
  listRemoteSkills,
  prepareRemoteSkills,
  getSkillDependencyOptions,
  listBuiltinSkills,
  syncBuiltinSkills,
  getSkillTree,
  getSkillFile,
  createSkillFile,
  updateSkillFile,
  updateSkillDependencies,
  updateSkillShareConfig,
  updateSkillEnabled,
  deleteSkillFile,
  exportSkill,
  deleteSkill,
  deleteSkillsBatch
}

export default skillApi
