const STATUS_MAP = {
  pending: { status: 'pending', progress: 0 },
  running: { status: 'running', progress: 50 },
  succeeded: { status: 'success', progress: 100 },
  failed: { status: 'failed', progress: 100 },
  cancelled: { status: 'cancelled', progress: 100 }
}

export const normalizeDocumentProcessingJob = (job = {}) => {
  const mapped = STATUS_MAP[job.status] || STATUS_MAP.pending
  const displayName = job.filename || job.file_id || '未知文件'
  const taskLabel = job.job_type === 'index' ? '文档入库' : '文档解析'
  const taskType = job.job_type === 'index' ? 'knowledge_index' : 'knowledge_parse'
  return {
    id: job.job_id,
    name: `${taskLabel} (${displayName})`,
    type: taskType,
    task_type: taskType,
    source: 'document_processing',
    status: mapped.status,
    progress: mapped.progress,
    message: job.error_message || '',
    error: job.error_message || '',
    created_at: job.created_at,
    updated_at: job.finished_at || job.started_at || job.created_at,
    started_at: job.started_at,
    completed_at: job.finished_at,
    payload: {
      kb_id: job.kb_id,
      file_id: job.file_id,
      filename: job.filename,
      job_type: job.job_type,
      attempt: job.attempt,
      max_attempts: job.max_attempts,
      error_code: job.error_code
    }
  }
}

export const mergeTaskSources = (legacyTasks = [], processingJobs = []) =>
  [...legacyTasks, ...processingJobs.map(normalizeDocumentProcessingJob)].sort((a, b) => {
    const timeA = Date.parse(a?.created_at || '') || 0
    const timeB = Date.parse(b?.created_at || '') || 0
    return timeB - timeA
  })

export const summarizeTasks = (tasks = []) => {
  const summary = {
    total: tasks.length,
    filtered_total: tasks.length,
    status_counts: {},
    type_counts: {}
  }
  for (const task of tasks) {
    const status = task?.status || 'pending'
    const type = task?.type || 'general'
    summary.status_counts[status] = (summary.status_counts[status] || 0) + 1
    summary.type_counts[type] = (summary.type_counts[type] || 0) + 1
  }
  return summary
}
