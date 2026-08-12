import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildDefaultExpandedSpanIds,
  buildTraceTree,
  buildTimelineRows,
  collectDiagnostics,
  collectTreeSpanIds,
  compressedIndent,
  normalizeDiagnostics
} from '../../src/utils/traceViewModel.js'

test('buildTraceTree keeps llm and tool as agent children', () => {
  const spans = [
    { span_id: 'agent', parent_span_id: '', operation: 'agent.run', started_at: '2026-08-08T00:00:00Z' },
    { span_id: 'llm', parent_span_id: 'agent', operation: 'llm.generate', started_at: '2026-08-08T00:00:01Z' },
    { span_id: 'tool', parent_span_id: 'agent', operation: 'tool.execute', started_at: '2026-08-08T00:00:02Z', attributes: { tool_call_id: 'call-1', triggered_by_span_id: 'llm' } }
  ]
  const roots = buildTraceTree(spans)
  assert.equal(roots.length, 1)
  assert.equal(roots[0].span_id, 'agent')
  assert.deepEqual(roots[0].children.map(x => x.span_id), ['llm', 'tool'])
  assert.equal(roots[0].children[1].attributes.triggered_by_span_id, 'llm')
})

test('buildTraceTree handles missing parent as orphan root', () => {
  const spans = [
    { span_id: 'a', parent_span_id: 'missing', operation: 'llm.generate', started_at: '2026-08-08T00:00:00Z' }
  ]
  const roots = buildTraceTree(spans)
  assert.equal(roots.length, 1)
  assert.equal(roots[0].span_id, 'a')
  assert.equal(roots[0].orphan, true)
})

test('buildTraceTree does not infinite loop on cycle', () => {
  const spans = [
    { span_id: 'a', parent_span_id: 'b', operation: 'llm.generate', started_at: '2026-08-08T00:00:00Z' },
    { span_id: 'b', parent_span_id: 'a', operation: 'tool.execute', started_at: '2026-08-08T00:00:01Z' }
  ]
  const roots = buildTraceTree(spans)
  // 循环节点应作为孤儿根节点出现，不递归死循环
  assert.ok(roots.length >= 1)
  assert.ok(roots.find(r => r.orphan === true))
})

test('buildTraceTree children sorted by started_at', () => {
  const spans = [
    { span_id: 'parent', parent_span_id: '', operation: 'agent.run', started_at: '2026-08-08T00:00:00Z' },
    { span_id: 'c', parent_span_id: 'parent', operation: 'tool.execute', started_at: '2026-08-08T00:00:03Z' },
    { span_id: 'a', parent_span_id: 'parent', operation: 'llm.generate', started_at: '2026-08-08T00:00:01Z' },
    { span_id: 'b', parent_span_id: 'parent', operation: 'llm.generate', started_at: '2026-08-08T00:00:02Z' }
  ]
  const roots = buildTraceTree(spans)
  assert.deepEqual(roots[0].children.map(x => x.span_id), ['a', 'b', 'c'])
})

test('buildDefaultExpandedSpanIds expands branches through depth one', () => {
  const tree = buildTraceTree([
    { span_id: 'root', parent_span_id: '', status: 'ok', started_at: '2026-08-08T00:00:00Z' },
    { span_id: 'agent', parent_span_id: 'root', status: 'ok', started_at: '2026-08-08T00:00:01Z' },
    { span_id: 'llm', parent_span_id: 'agent', status: 'ok', started_at: '2026-08-08T00:00:02Z' },
    { span_id: 'tool', parent_span_id: 'llm', status: 'ok', started_at: '2026-08-08T00:00:03Z' }
  ])

  assert.deepEqual(buildDefaultExpandedSpanIds(tree), ['root', 'agent'])
})

test('buildDefaultExpandedSpanIds expands every ancestor of an error span', () => {
  const tree = buildTraceTree([
    { span_id: 'root', parent_span_id: '', status: 'ok', started_at: '2026-08-08T00:00:00Z' },
    { span_id: 'agent', parent_span_id: 'root', status: 'ok', started_at: '2026-08-08T00:00:01Z' },
    { span_id: 'llm', parent_span_id: 'agent', status: 'ok', started_at: '2026-08-08T00:00:02Z' },
    { span_id: 'tool', parent_span_id: 'llm', status: 'error', started_at: '2026-08-08T00:00:03Z' }
  ])

  assert.deepEqual(buildDefaultExpandedSpanIds(tree), ['root', 'agent', 'llm'])
})

test('collectTreeSpanIds returns stable unique ids and handles empty input', () => {
  const tree = buildTraceTree([
    { span_id: 'root', parent_span_id: '', started_at: '2026-08-08T00:00:00Z' },
    { span_id: 'child', parent_span_id: 'root', started_at: '2026-08-08T00:00:01Z' }
  ])

  assert.deepEqual(collectTreeSpanIds(tree), ['root', 'child'])
  assert.deepEqual(collectTreeSpanIds([]), [])
})

test('buildTimelineRows computes leftPct and widthPct with min 0.4%', () => {
  const spans = [
    { span_id: 's1', started_at: '2026-08-08T00:00:00Z', ended_at: '2026-08-08T00:00:02Z' },
    { span_id: 's2', started_at: '2026-08-08T00:00:01Z', ended_at: '2026-08-08T00:00:01.001Z' }
  ]
  const rows = buildTimelineRows(spans)
  assert.equal(rows.length, 2)
  // s1 占据 0-2s，总时长 2s，leftPct=0, widthPct=100
  assert.ok(rows[0].leftPct >= 0)
  assert.ok(rows[0].widthPct > 0)
  // s2 极短，widthPct 至少 0.4
  assert.ok(rows[1].widthPct >= 0.4, `widthPct=${rows[1].widthPct} should be >= 0.4`)
})

test('buildTimelineRows handles missing ended_at', () => {
  const spans = [
    { span_id: 's1', started_at: '2026-08-08T00:00:00Z' }
  ]
  const rows = buildTimelineRows(spans)
  assert.equal(rows.length, 1)
  assert.ok(rows[0].widthPct >= 0.4)
})

test('collectDiagnostics reports orphan and running span', () => {
  const diagnostics = collectDiagnostics([{ span_id: 'x', parent_span_id: 'missing', status: 'running' }])
  assert.deepEqual(diagnostics.map(x => x.code).sort(), ['orphan_span', 'running_span'])
})

test('collectDiagnostics ignores http.server running span', () => {
  const diagnostics = collectDiagnostics([
    { span_id: 'http', parent_span_id: '', status: 'running', operation: 'http.server' }
  ])
  assert.equal(diagnostics.length, 0)
})

test('collectDiagnostics reports status_mismatch', () => {
  const diagnostics = collectDiagnostics(
    [{ span_id: 'agent', parent_span_id: '', status: 'ok', operation: 'agent.run' }],
    { status: 'failed' }
  )
  assert.ok(diagnostics.find(d => d.code === 'status_mismatch'))
})

test('collectDiagnostics reports slow_queue when queue wait exceeds threshold', () => {
  const diagnostics = collectDiagnostics(
    [{ span_id: 'qc', parent_span_id: '', status: 'ok', operation: 'queue.consume', attributes: { queue_wait_ms: 30000 } }],
    null,
    { slowQueueMs: 5000 }
  )
  assert.ok(diagnostics.find(d => d.code === 'slow_queue'))
})

test('normalizeDiagnostics keeps backend diagnostics in the frontend shape', () => {
  const diagnostics = normalizeDiagnostics([
    { code: 'dropped_data', message: 'writer dropped 2 events', span_id: 'span-1' }
  ])

  assert.deepEqual(diagnostics, [
    { code: 'dropped_data', message: 'writer dropped 2 events', span_id: 'span-1' }
  ])
})

test('normalizeDiagnostics ignores invalid backend diagnostics', () => {
  const diagnostics = normalizeDiagnostics([
    null,
    { code: 'slow_queue', message: '' },
    { code: 'running_span', message: 'still running' }
  ])

  assert.deepEqual(diagnostics, [
    { code: 'running_span', message: 'still running', span_id: '' }
  ])
  assert.deepEqual(normalizeDiagnostics(null), [])
})

test('compressedIndent keeps full step before compactAfter and compact step after', () => {
  assert.equal(compressedIndent(0, 18, 6, 8), 0)
  assert.equal(compressedIndent(8, 18, 6, 8), 144)
  assert.equal(compressedIndent(9, 18, 6, 8), 150)
  assert.equal(compressedIndent(30, 18, 6, 8), 276)
  assert.equal(compressedIndent(30, 14, 6, 10), 260)
  assert.equal(compressedIndent(-3, 18, 6, 8), 0)
})
