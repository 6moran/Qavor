/**
 * Trace 详情页面纯函数数据转换。
 * 将后端返回的平铺 Span 列表转换为前端可直接渲染的调用树、时间线行和诊断提示。
 */

/**
 * 将平铺的 Span 列表组装为调用树。
 * - 缺失父 Span 的节点作为孤儿根节点（orphan=true）
 * - 检测循环引用，循环节点断开为孤儿根节点，避免递归死循环
 * - 子节点按 started_at 升序排序
 * @param {Array} spans - 平铺的 Span 列表
 * @returns {Array} 根节点数组，每个节点含 children 数组
 */
export function buildTraceTree(spans) {
  if (!spans || spans.length === 0) return []

  const byId = new Map()
  spans.forEach(s => byId.set(s.span_id, { ...s, children: [], orphan: false }))

  const roots = []

  // 检测循环引用：沿 parent 链上溯，若回到自身则存在环
  const hasCycleToRoot = (node, targetId) => {
    let cur = node
    let depth = 0
    const maxDepth = byId.size + 1
    while (cur && cur.parent_span_id && depth < maxDepth) {
      if (cur.parent_span_id === targetId) return true
      cur = byId.get(cur.parent_span_id)
      depth++
    }
    return false
  }

  byId.forEach(node => {
    if (!node.parent_span_id) {
      roots.push(node)
      return
    }
    const parent = byId.get(node.parent_span_id)
    if (!parent) {
      // 父节点不存在 -> 孤儿根节点
      node.orphan = true
      roots.push(node)
      return
    }
    // 检测循环：如果从当前节点上溯能回到自身，断开为孤儿
    if (hasCycleToRoot(node, node.span_id)) {
      node.orphan = true
      roots.push(node)
      return
    }
    parent.children.push(node)
  })

  // 子节点按 started_at 升序排序
  const sortChildren = node => {
    node.children.sort((a, b) => {
      const ta = new Date(a.started_at).getTime()
      const tb = new Date(b.started_at).getTime()
      return ta - tb
    })
    node.children.forEach(sortChildren)
  }
  roots.sort((a, b) => {
    const ta = new Date(a.started_at).getTime()
    const tb = new Date(b.started_at).getTime()
    return ta - tb
  })
  roots.forEach(sortChildren)

  return roots
}

const DEFAULT_ERROR_STATUSES = new Set(['error', 'timeout', 'cancelled', 'interrupted'])

/**
 * 按树的稳定遍历顺序收集全部 Span ID。
 * @param {Array} roots - buildTraceTree 返回的根节点数组
 * @returns {Array<string>}
 */
export function collectTreeSpanIds(roots) {
  if (!Array.isArray(roots) || roots.length === 0) return []

  const ids = []
  const visited = new Set()
  const visit = node => {
    if (!node?.span_id || visited.has(node.span_id)) return
    visited.add(node.span_id)
    ids.push(node.span_id)
    const children = Array.isArray(node.children) ? node.children : []
    children.forEach(visit)
  }

  roots.forEach(visit)
  return ids
}

/**
 * 计算调用树首次展示时需要展开的分支。
 * 默认展开深度 0、1 的有子节点节点，并补齐异常 Span 的全部祖先路径。
 * @param {Array} roots - buildTraceTree 返回的根节点数组
 * @param {Object} options - maxDepth 与 errorStatuses
 * @returns {Array<string>}
 */
export function buildDefaultExpandedSpanIds(roots, options = {}) {
  if (!Array.isArray(roots) || roots.length === 0) return []

  const maxDepth = options.maxDepth ?? 1
  const errorStatuses = new Set(options.errorStatuses || DEFAULT_ERROR_STATUSES)
  const expanded = new Set()
  const visited = new Set()

  const visit = (node, depth, ancestors) => {
    if (!node?.span_id || visited.has(node.span_id)) return
    visited.add(node.span_id)
    const children = Array.isArray(node.children) ? node.children : []

    if (children.length > 0 && depth <= maxDepth) expanded.add(node.span_id)
    if (errorStatuses.has(node.status)) ancestors.forEach(id => expanded.add(id))

    children.forEach(child => visit(child, depth + 1, [...ancestors, node.span_id]))
  }

  roots.forEach(root => visit(root, 0, []))
  return collectTreeSpanIds(roots).filter(id => expanded.has(id))
}

/**
 * 计算指定深度的压缩缩进像素值。
 * 前 compactAfter 层每层 step px，之后每层 compactStep px，
 * 避免深层 Span 把内容推出可视区。
 * @param {number} depth - 节点深度，负值按 0 处理
 * @param {number} step - 常规层递进像素
 * @param {number} compactStep - 深层递进像素
 * @param {number} compactAfter - 超过该深度后启用紧凑递进
 * @returns {number}
 */
export function compressedIndent(depth, step, compactStep, compactAfter) {
  const safeDepth = Math.max(depth, 0)
  const full = Math.min(safeDepth, compactAfter) * step
  const extra = Math.max(safeDepth - compactAfter, 0) * compactStep
  return full + extra
}

/**
 * 计算时间线行（瀑布图），leftPct/widthPct 归一化到 0-100，最短 0.4%。
 * @param {Array} spans - 平铺的 Span 列表
 * @returns {Array} 行数组，每项含 span 原始字段 + leftPct + widthPct
 */
export function buildTimelineRows(spans) {
  if (!spans || spans.length === 0) return []

  let minStart = Infinity
  let maxEnd = -Infinity
  spans.forEach(s => {
    const st = new Date(s.started_at).getTime()
    if (!Number.isNaN(st)) minStart = Math.min(minStart, st)
    const en = s.ended_at ? new Date(s.ended_at).getTime() : st
    if (!Number.isNaN(en)) maxEnd = Math.max(maxEnd, en)
  })
  if (minStart === Infinity || maxEnd === -Infinity) {
    minStart = Date.now()
    maxEnd = minStart + 1
  }
  const totalMs = Math.max(maxEnd - minStart, 1)

  return spans.map(span => {
    const st = new Date(span.started_at).getTime()
    const en = span.ended_at ? new Date(span.ended_at).getTime() : st
    const startMs = Number.isNaN(st) ? minStart : st
    const endMs = Number.isNaN(en) ? startMs : en
    const leftPct = Math.max(((startMs - minStart) / totalMs) * 100, 0)
    let widthPct = ((endMs - startMs) / totalMs) * 100
    if (widthPct < 0.4) widthPct = 0.4
    // leftPct + widthPct 不超过 100
    const clampedLeft = Math.min(leftPct, 100 - widthPct)
    // 计算实际耗时（毫秒），优先使用 started_at/ended_at，回退到 duration_ms
    const actualDurationMs = endMs > startMs ? (endMs - startMs) : (span.duration_ms || 0)
    return { ...span, leftPct: clampedLeft, widthPct, actualDurationMs }
  })
}

const EXTRA_TIMELINE_BAR_COLORS = {
  context: '#722ed1',
  embedding: '#08979c',
  rerank: '#d4380d'
}

/**
 * 返回未包含在基础瀑布样式中的 Span 颜色；错误状态始终使用红色。
 * @param {string} kind - Span 类型
 * @param {string} status - Span 状态
 * @returns {string|undefined}
 */
export function timelineBarColor(kind, status) {
  if (status === 'error') return '#cf1322'
  return EXTRA_TIMELINE_BAR_COLORS[kind]
}

/**
 * 收集诊断提示，输出稳定的 code 列表。
 * code: running_span / orphan_span / status_mismatch / dropped_data / slow_queue
 * @param {Array} spans - 平铺的 Span 列表
 * @param {Object|null} runSummary - 关联 Run 摘要 { status }
 * @param {Object} options - { slowQueueMs } 阈值，默认 10000ms
 * @returns {Array} 诊断数组，每项含 code, message, span_id
 */
export function collectDiagnostics(spans, runSummary = null, options = {}) {
  if (!spans || spans.length === 0) return []
  const slowQueueMs = options.slowQueueMs ?? 10000
  const diags = []

  const spanIds = new Set(spans.map(s => s.span_id))

  spans.forEach(s => {
    // running_span：非 http.server 的 running span
    if (s.status === 'running' && s.operation !== 'http.server') {
      diags.push({
        code: 'running_span',
        message: `span ${s.span_id} (${s.operation || s.kind}) 仍处于 running 状态`,
        span_id: s.span_id
      })
    }
    // orphan_span：parent_span_id 非空但不存在
    if (s.parent_span_id && !spanIds.has(s.parent_span_id)) {
      diags.push({
        code: 'orphan_span',
        message: `span ${s.span_id} 的父 span ${s.parent_span_id} 不存在`,
        span_id: s.span_id
      })
    }
    // slow_queue：queue.consume 排队时间超过阈值
    if (s.operation === 'queue.consume' && s.attributes) {
      const waitMs = s.attributes.queue_wait_ms
      if (typeof waitMs === 'number' && waitMs > slowQueueMs) {
        diags.push({
          code: 'slow_queue',
          message: `排队等待 ${waitMs}ms 超过阈值 ${slowQueueMs}ms`,
          span_id: s.span_id
        })
      }
    }
  })

  // status_mismatch：agent.run span 状态与 agent_runs.status 不一致
  if (runSummary && runSummary.status) {
    const agentRun = spans.find(s => s.operation === 'agent.run')
    if (agentRun && !isStatusConsistent(agentRun.status, runSummary.status)) {
      diags.push({
        code: 'status_mismatch',
        message: `agent.run span 状态为 ${agentRun.status}，但 agent_runs.status 为 ${runSummary.status}`,
        span_id: agentRun.span_id
      })
    }
  }

  return diags
}

/**
 * 将后端诊断结果收敛为页面使用的稳定结构。
 * 后端 diagnostics 为空或格式异常时，调用方可以回退到 collectDiagnostics。
 * @param {unknown} diagnostics
 * @returns {Array<{code: string, message: string, span_id: string}>}
 */
export function normalizeDiagnostics(diagnostics) {
  if (!Array.isArray(diagnostics)) return []

  return diagnostics
    .filter(item => item && typeof item === 'object')
    .map(item => ({
      code: typeof item.code === 'string' ? item.code : '',
      message: typeof item.message === 'string' ? item.message : '',
      span_id: typeof item.span_id === 'string' ? item.span_id : ''
    }))
    .filter(item => item.code && item.message)
}

/**
 * 判断 agent.run span 状态与业务 Run 状态是否一致。
 * ok ↔ completed, error ↔ failed, cancelled ↔ cancelled, interrupted ↔ interrupted, running ↔ pending/running
 */
function isStatusConsistent(agentStatus, businessStatus) {
  const mapping = {
    ok: ['completed'],
    error: ['failed'],
    // 用户手动中断：span 常落成 cancelled 而业务 run 是 interrupted，互认避免误报
    cancelled: ['cancelled', 'interrupted'],
    interrupted: ['interrupted', 'cancelled'],
    running: ['pending', 'running'],
    timeout: ['failed', 'cancelled']
  }
  const allowed = mapping[agentStatus]
  if (!allowed) return true // 未知状态不报不一致
  return allowed.includes(businessStatus)
}
