export function buildKnowledgeBaseCreateRequest(form) {
  const request = {
    database_name: form.name?.trim() || '',
    description: form.description?.trim() || '',
    embedding_model_id: form.embedding_model_id,
    chat_model_id: form.chat_model_id,
    additional_params: {}
  }

  if (form.chunk_preset_id) {
    request.additional_params.chunk_preset_id = form.chunk_preset_id
  }

  return request
}
