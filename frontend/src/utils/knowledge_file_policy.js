export const FILE_ACTIONS = {
  PARSE: 'parse',
  INDEX: 'index'
}

const STATUS_VIEW = {
  uploaded: { label: '等待解析', tone: 'status-warning', icon: 'clock' },
  parsing: { label: '解析中', tone: 'status-info', icon: 'progress' },
  parsed: { label: '待入库', tone: 'status-primary', icon: 'file' },
  error_parsing: { label: '重试解析', tone: 'status-error', icon: 'error' },
  indexing: { label: '入库中', tone: 'status-info', icon: 'progress' },
  indexed: { label: '已入库', tone: 'status-success', icon: 'success' },
  index_failed: { label: '入库失败', tone: 'status-error', icon: 'error' },
  error_indexing: { label: '重试入库', tone: 'status-error', icon: 'error' },
  done: { label: '已入库', tone: 'status-success', icon: 'success' },
  failed: { label: '解析失败', tone: 'status-error', icon: 'error' },
  processing: { label: '解析中', tone: 'status-info', icon: 'progress' },
  ready: { label: '待入库', tone: 'status-primary', icon: 'file' },
  waiting: { label: '等待中', tone: 'status-warning', icon: 'clock' }
}

const STATUS_ACTION = {
  error_parsing: { type: FILE_ACTIONS.PARSE, label: '重试解析' },
  failed: { type: FILE_ACTIONS.PARSE, label: '重试解析' },
  parsed: { type: FILE_ACTIONS.INDEX, label: '入库' },
  ready: { type: FILE_ACTIONS.INDEX, label: '入库' },
  index_failed: { type: FILE_ACTIONS.INDEX, label: '重试入库' },
  error_indexing: { type: FILE_ACTIONS.INDEX, label: '重试入库' }
}

const PARSED_PREVIEW_STATUSES = new Set([
  'done',
  'parsed',
  'ready',
  'indexed',
  'index_failed',
  'error_indexing'
])
const TABLE_SELECTION_BLOCKED_STATUSES = new Set(['processing', 'waiting'])
const DELETE_BLOCKED_STATUSES = new Set(['processing', 'parsing', 'indexing'])
const PROCESSING_STATUSES = new Set(['uploaded', 'processing', 'waiting', 'parsing', 'indexing'])
const INDEXABLE_STATUSES = new Set([
  'parsed',
  'ready',
  'index_failed',
  'error_indexing',
  'done',
  'indexed'
])
const PARSEABLE_STATUSES = new Set(['failed', 'error_parsing'])
const CHUNK_PREVIEW_STATUSES = new Set(['done', 'indexed'])
const STATUS_SORT_ORDER = {
  done: 1,
  indexed: 1,
  processing: 2,
  ready: 1,
  indexing: 2,
  parsing: 2,
  waiting: 3,
  uploaded: 3,
  parsed: 3,
  failed: 4,
  index_failed: 4,
  error_indexing: 4,
  error_parsing: 4
}

export const FILE_STATUS_FILTER_OPTIONS = [
  { label: '等待解析', value: 'uploaded' },
  { label: '解析中', value: 'processing' },
  { label: '待入库', value: 'ready' },
  { label: '解析失败', value: 'failed' },
  { label: '入库中', value: 'indexing' },
  { label: '已入库', value: 'indexed' },
  { label: '入库失败', value: 'index_failed' }
]

export const getFileStatusView = (status) =>
  STATUS_VIEW[status] || { label: status || '', tone: '', icon: null }

export const getFilePrimaryAction = (record) => {
  if (!record || record.is_folder) return null
  return STATUS_ACTION[record.status] || null
}

export const canParseFile = (record) =>
  Boolean(record && !record.is_folder && PARSEABLE_STATUSES.has(record.status))

export const canIndexFile = (record) =>
  Boolean(record && !record.is_folder && INDEXABLE_STATUSES.has(record.status))

export const canReindexFile = (record) =>
  Boolean(record && !record.is_folder && (record.status === 'done' || record.status === 'indexed'))

export const canDownloadFile = (record) =>
  Boolean(
    record &&
    !record.is_folder &&
    record.file_type !== 'url' &&
    ('has_original_file' in record ? record.has_original_file : Boolean(record.path))
  )

export const canSelectFile = (record, locked = false) =>
  Boolean(
    record && !record.is_folder && !locked && !TABLE_SELECTION_BLOCKED_STATUSES.has(record.status)
  )

export const canDeleteFile = (record, locked = false) =>
  Boolean(record && !record.is_folder && !locked && !DELETE_BLOCKED_STATUSES.has(record.status))

export const isProcessingFile = (record) =>
  Boolean(record && PROCESSING_STATUSES.has(record.status))

export const matchesStatusFilter = (record, status) => {
  if (!record || status === 'all') return true
  return (
    record.status === status ||
    (status === 'ready' && record.status === 'parsed') ||
    (status === 'indexed' && record.status === 'done') ||
    (status === 'failed' && record.status === 'error_parsing') ||
    (status === 'index_failed' && record.status === 'error_indexing')
  )
}

export const getFileStatusSortWeight = (record) => STATUS_SORT_ORDER[record?.status] || 5

export const canPreviewParsed = (record) => {
  if (!record || record.is_folder) return false
  if ('has_parsed_markdown' in record) return Boolean(record.has_parsed_markdown)
  return PARSED_PREVIEW_STATUSES.has(record.status)
}

export const canPreviewOriginal = (record) => {
  if (!record || record.is_folder || record.file_type === 'url') return false
  if ('has_original_file' in record) return Boolean(record.has_original_file)
  return true
}

export const canPreviewChunks = (record) =>
  Boolean(record && !record.is_folder && CHUNK_PREVIEW_STATUSES.has(record.status))

export const canOpenFileDetail = (record) =>
  canPreviewParsed(record) ||
  canPreviewOriginal(record)

export const getDefaultDetailView = (record) => {
  if (!canPreviewParsed(record) && canPreviewOriginal(record)) return 'source'
  return 'markdown'
}
