export const buildIndexDocumentsRequest = (kbId, fileIds, params = {}) => {
  const body = {
    file_ids: fileIds,
    params: {
      chunk_preset_id: params.chunk_preset_id || 'general',
      chunk_parser_config: {
        chunk_token_num: params.chunk_parser_config?.chunk_token_num || 500,
        overlapped_percent: params.chunk_parser_config?.overlapped_percent ?? 10
      }
    }
  }
  return {
    url: `/api/v1/knowledge/databases/${encodeURIComponent(kbId)}/documents/index`,
    body
  }
}
