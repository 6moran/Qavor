export const FILE_ACTIONS = {
  PARSE: 'parse',
  INDEX: 'index'
}

// 旧版到标准状态的兼容性规范化器。
// 生产数据迁移确认后，可移除此函数及相关别名。
const normalizeStatus = (status) => {
  const aliases = {
    processing: 'parsing',
    ready: 'parsed',
    done: 'indexed',
    failed: 'parse_failed',
    error_parsing: 'parse_failed',
    error_indexing: 'index_failed',
    waiting: 'parse_queued'
  }
  return aliases[status] || status
}

const STATUS_VIEW = {
  uploaded: { label: '已上传', tone: 'status-warning', icon: 'clock' },
  parse_queued: { label: '等待解析', tone: 'status-warning', icon: 'clock' },
  parsing: { label: '解析中', tone: 'status-info', icon: 'progress' },
  parsed: { label: '待入库', tone: 'status-primary', icon: 'file' },
  parse_failed: { label: '解析失败', tone: 'status-error', icon: 'error' },
  index_queued: { label: '等待入库', tone: 'status-warning', icon: 'clock' },
  indexing: { label: '入库中', tone: 'status-info', icon: 'progress' },
  indexed: { label: '已入库', tone: 'status-success', icon: 'success' },
  index_failed: { label: '入库失败', tone: 'status-error', icon: 'error' }
}

const STATUS_ACTION = {
  parse_failed: { type: FILE_ACTIONS.PARSE, label: '重试解析' },
  parsed: { type: FILE_ACTIONS.INDEX, label: '入库' },
  index_failed: { type: FILE_ACTIONS.INDEX, label: '重新入库' },
  indexed: { type: FILE_ACTIONS.INDEX, label: '重新入库' }
}

const PARSED_PREVIEW_STATUSES = new Set([
  'parsed',
  'index_queued',
  'indexing',
  'indexed',
  'index_failed'
])
const TABLE_SELECTION_BLOCKED_STATUSES = new Set(['parse_queued', 'parsing', 'index_queued', 'indexing'])
const DELETE_BLOCKED_STATUSES = new Set(['parsing', 'indexing'])
const PROCESSING_STATUSES = new Set(['uploaded', 'parse_queued', 'parsing', 'index_queued', 'indexing'])
const INDEXABLE_STATUSES = new Set(['parsed', 'index_failed', 'indexed'])
const PARSEABLE_STATUSES = new Set(['parse_failed'])
const CHUNK_PREVIEW_STATUSES = new Set(['indexed'])
const STATUS_SORT_ORDER = {
  indexed: 1,
  index_failed: 1,
  indexing: 2,
  parsing: 2,
  parse_queued: 3,
  index_queued: 3,
  uploaded: 3,
  parsed: 3,
  parse_failed: 4
}

export const FILE_STATUS_FILTER_OPTIONS = [
  { label: '等待解析', value: 'parse_queued' },
  { label: '解析中', value: 'parsing' },
  { label: '待入库', value: 'parsed' },
  { label: '解析失败', value: 'parse_failed' },
  { label: '入库中', value: 'indexing' },
  { label: '已入库', value: 'indexed' },
  { label: '入库失败', value: 'index_failed' }
]

export const getFileStatusView = (status) => {
  const canonical = normalizeStatus(status)
  return STATUS_VIEW[canonical] || { label: status || '', tone: '', icon: null }
}

export const getFilePrimaryAction = (record) => {
  if (!record || record.is_folder) return null
  const canonical = normalizeStatus(record.status)
  return STATUS_ACTION[canonical] || null
}

export const canParseFile = (record) => {
  const canonical = normalizeStatus(record?.status)
  return Boolean(record && !record.is_folder && PARSEABLE_STATUSES.has(canonical))
}

export const canIndexFile = (record) => {
  const canonical = normalizeStatus(record?.status)
  return Boolean(record && !record.is_folder && INDEXABLE_STATUSES.has(canonical))
}

export const canReparseFile = (record) => {
  const canonical = normalizeStatus(record?.status)
  return Boolean(record && !record.is_folder && canonical === 'indexed')
}

export const canDownloadFile = (record) =>
  Boolean(
    record &&
    !record.is_folder &&
    record.file_type !== 'url' &&
    ('has_original_file' in record ? record.has_original_file : Boolean(record.path))
  )

export const canSelectFile = (record, locked = false) => {
  const canonical = normalizeStatus(record?.status)
  return Boolean(
    record && !record.is_folder && !locked && !TABLE_SELECTION_BLOCKED_STATUSES.has(canonical)
  )
}

export const canDeleteFile = (record, locked = false) => {
  const canonical = normalizeStatus(record?.status)
  return Boolean(record && !record.is_folder && !locked && !DELETE_BLOCKED_STATUSES.has(canonical))
}

export const isProcessingFile = (record) => {
  const canonical = normalizeStatus(record?.status)
  return Boolean(record && PROCESSING_STATUSES.has(canonical))
}

export const matchesStatusFilter = (record, status) => {
  if (!record || status === 'all') return true
  const canonical = normalizeStatus(record.status)
  return canonical === status
}

export const getFileStatusSortWeight = (record) => {
  const canonical = normalizeStatus(record?.status)
  return STATUS_SORT_ORDER[canonical] || 5
}

export const canPreviewParsed = (record) => {
  if (!record || record.is_folder) return false
  if ('has_parsed_markdown' in record) return Boolean(record.has_parsed_markdown)
  const canonical = normalizeStatus(record.status)
  return PARSED_PREVIEW_STATUSES.has(canonical)
}

export const canPreviewOriginal = (record) => {
  if (!record || record.is_folder || record.file_type === 'url') return false
  if ('has_original_file' in record) return Boolean(record.has_original_file)
  return true
}

export const canPreviewChunks = (record) => {
  const canonical = normalizeStatus(record?.status)
  return Boolean(record && !record.is_folder && CHUNK_PREVIEW_STATUSES.has(canonical))
}

export const canOpenFileDetail = (record) =>
  canPreviewParsed(record) ||
  canPreviewOriginal(record)

export const getDefaultDetailView = (record) => {
  if (!canPreviewParsed(record) && canPreviewOriginal(record)) return 'source'
  return 'markdown'
}
