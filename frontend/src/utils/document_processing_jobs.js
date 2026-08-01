const TERMINAL_PROCESSING_STATUSES = new Set(['succeeded', 'failed', 'cancelled'])

export const isTerminalProcessingStatus = (status) =>
  TERMINAL_PROCESSING_STATUSES.has(String(status || '').toLowerCase())

const defaultSleep = (duration) => new Promise((resolve) => setTimeout(resolve, duration))

export async function pollDocumentProcessingJobs(jobIds, options = {}) {
  const ids = [...new Set((jobIds || []).filter(Boolean))]
  if (ids.length === 0) return []

  const {
    fetchJob,
    intervalMs = 1000,
    maxRounds = 300,
    sleep = defaultSleep,
    onUpdate
  } = options

  if (typeof fetchJob !== 'function') {
    throw new TypeError('fetchJob must be a function')
  }

  const pending = new Set(ids)
  const results = new Map()

  for (let round = 0; round < maxRounds; round += 1) {
    const updates = await Promise.all([...pending].map((jobId) => fetchJob(jobId)))
    for (const job of updates) {
      const jobId = job?.job_id
      if (!jobId) {
        throw new Error('文档处理任务接口缺少 job_id')
      }
      results.set(jobId, job)
      if (isTerminalProcessingStatus(job.status)) {
        pending.delete(jobId)
      }
    }

    if (typeof onUpdate === 'function') {
      await onUpdate(ids.map((jobId) => results.get(jobId)).filter(Boolean))
    }
    if (pending.size === 0) {
      return ids.map((jobId) => results.get(jobId))
    }
    if (round < maxRounds - 1) {
      await sleep(intervalMs)
    }
  }

  const error = new Error('等待文档解析结果超时，请稍后在文件列表中查看')
  error.code = 'DOCUMENT_PROCESSING_TIMEOUT'
  error.jobs = ids.map((jobId) => results.get(jobId)).filter(Boolean)
  throw error
}
