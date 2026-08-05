export const DEFAULT_MODEL_PARAMS = {
  max_tokens: 4096,
  temperature: 0.7,
  top_p: 1,
  presence_penalty: 0,
  frequency_penalty: 0,
  stop: []
}

export const MODEL_PROTOCOL_OPTIONS = [{ label: 'OpenAI', value: 'openai' }]

export const formatJsonText = (value) => JSON.stringify(value ?? {}, null, 2)

export const createDefaultModelForm = () => ({
  name: '',
  remark: '',
  protocol: 'openai',
  base_url: '',
  api_key: '',
  headers: formatJsonText({}),
  timeout: 30000,
  enabled: true,
  model_type: 'chat',
  params: formatJsonText(DEFAULT_MODEL_PARAMS)
})

export const parseJsonObject = (text, label) => {
  let parsed
  try {
    parsed = JSON.parse(typeof text === 'string' && text.trim() ? text : '{}')
  } catch (error) {
    throw new Error(`${label}格式不正确`, { cause: error })
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error(`${label}必须是 JSON 对象`)
  }
  return parsed
}

export const buildModelPayload = (form) => ({
  name: form.name.trim(),
  remark: form.remark?.trim() || '',
  protocol: form.protocol.trim(),
  base_url: form.base_url.trim(),
  api_key: form.api_key || '',
  headers: parseJsonObject(form.headers, '请求头'),
  timeout: Number(form.timeout) || 30000,
  enabled: Boolean(form.enabled),
  model_type: form.model_type,
  params: parseJsonObject(form.params, '参数')
})

export const buildModelTestPayload = (form, modelId = null) => {
  const payload = buildModelPayload(form)
  const result = {
    name: payload.name,
    protocol: payload.protocol,
    base_url: payload.base_url,
    api_key: payload.api_key,
    timeout: payload.timeout,
    model_type: payload.model_type
  }
  if (modelId) result.model_id = modelId
  return result
}

export const formatModelTestSuccess = ({ latency_ms } = {}) =>
  `连接成功 · ${Number(latency_ms) || 0} ms`

export const modelToForm = (model) => ({
  name: model.name || '',
  remark: model.remark || '',
  protocol: model.protocol || 'openai',
  base_url: model.base_url || '',
  api_key: '',
  headers: formatJsonText(model.headers),
  timeout: model.timeout || 30000,
  enabled: model.enabled !== false,
  model_type: model.model_type || 'chat',
  params: formatJsonText(model.params || DEFAULT_MODEL_PARAMS)
})

export const resetAdvancedFields = () => ({
  headers: formatJsonText({}),
  params: formatJsonText(DEFAULT_MODEL_PARAMS)
})
