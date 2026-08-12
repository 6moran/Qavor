const normalizeModelId = (value) => {
  if (value === null || value === undefined || value === '') return null
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

export const normalizeRagSettingsResponse = (response) => {
  const data = response?.data && typeof response.data === 'object' ? response.data : response || {}
  return {
    rerankModelId: normalizeModelId(data.rerank_model_id),
    rerankModelName:
      typeof data.rerank_model_name === 'string' ? data.rerank_model_name : ''
  }
}

export const buildRagSettingsPayload = (rerankModelId) => ({
  rerank_model_id: normalizeModelId(rerankModelId)
})

export const persistRerankSelection = async ({ previous, nextModelId, update }) => {
  try {
    const response = await update(normalizeModelId(nextModelId))
    return { settings: normalizeRagSettingsResponse(response), error: null }
  } catch (error) {
    return { settings: previous, error }
  }
}
