import test from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const source = readFileSync(new URL('../../src/views/TraceListView.vue', import.meta.url), 'utf8')
const routerSource = readFileSync(new URL('../../src/router/index.js', import.meta.url), 'utf8')
const detailUrl = new URL('../../src/views/TraceDetailView.vue', import.meta.url)
const panelUrl = new URL('../../src/components/trace/TraceDetailPanel.vue', import.meta.url)

test('Trace list binds row navigation through Ant Table customRow', () => {
  assert.match(source, /:custom-row="resolveTraceRow"/)
  assert.doesNotMatch(source, /@row-click=/)
})

test('Trace list exposes a visible detail action', () => {
  assert.match(source, /column\.key === 'action'/)
  assert.match(source, />查看详情</)
})

test('Trace list navigates to a standalone detail page', () => {
  assert.doesNotMatch(source, /<a-drawer/)
  assert.match(source, /name:\s*'TraceDetailComp'/)
  assert.match(source, /trace_id:\s*record\.trace_id/)
})

test('Trace detail is registered as a standalone route with a back button', () => {
  assert.match(routerSource, /path:\s*':trace_id'/)
  assert.match(routerSource, /TraceDetailComp/)
  assert.equal(existsSync(fileURLToPath(detailUrl)), true)
  const detailSource = readFileSync(detailUrl, 'utf8')
  assert.match(detailSource, />\s*返回\s*</)
  assert.match(detailSource, /TraceDetailPanel/)
})

test('Trace detail panel loads only from its traceId prop', () => {
  assert.equal(existsSync(fileURLToPath(panelUrl)), true)
  const panelSource = readFileSync(panelUrl, 'utf8')
  assert.match(panelSource, /defineProps/)
  assert.match(panelSource, /traceId/)
  assert.match(panelSource, /traceApi\.getTrace\(props\.traceId\)/)
  assert.doesNotMatch(panelSource, /useRoute|PageHeader/)
})

test('Trace detail panel implements prototype D timeline tree and side detail layout', () => {
  const panelSource = readFileSync(panelUrl, 'utf8')
  assert.doesNotMatch(panelSource, /<style scoped>/)
  assert.match(panelSource, /class="trace-timeline"/)
  assert.match(panelSource, /class="trace-tree-pane"/)
  assert.match(panelSource, /class="trace-span-detail"/)
  assert.match(panelSource, /selectSpan/)
  assert.match(panelSource, /class: \['tree-node-row'/)
  assert.match(panelSource, /class: 'tree-branch-toggle'/)
})

test('Trace tree keeps type colors for LLM, tools, and agents', () => {
  const panelSource = readFileSync(panelUrl, 'utf8')
  assert.match(panelSource, /wf-kind-\$\{(?:props|treeProps)\.node\.kind\}/)
  assert.match(panelSource, /\.wf-kind-llm\s*\{[^}]*#1677ff/s)
  assert.match(panelSource, /\.wf-kind-tool\s*\{[^}]*#52c41a/s)
  assert.match(panelSource, /\.wf-kind-agent\s*\{[^}]*#fa8c16/s)
})

test('Selected trace tree row has a strong visual highlight', () => {
  const panelSource = readFileSync(panelUrl, 'utf8')
  assert.match(panelSource, /tree-node-row--selected/)
  assert.match(panelSource, /\.tree-node-row--selected\s*\{[^}]*box-shadow:\s*inset 3px 0/s)
  assert.match(panelSource, /\.tree-node-row--selected\s*\{[^}]*outline:/s)
})

test('Trace panel defaults to fully expanded tree like the timeline', () => {
  const panelSource = readFileSync(panelUrl, 'utf8')
  assert.match(panelSource, /collectTreeSpanIds\(treeRoots\.value\)\.forEach\(id => expandedBranches\.add\(id\)\)/)
  assert.doesNotMatch(panelSource, /buildDefaultExpandedSpanIds\(treeRoots\.value\)/)
})

test('Trace tree and timeline use compressed indent with horizontal scroll fallback', () => {
  const panelSource = readFileSync(panelUrl, 'utf8')
  assert.match(panelSource, /compressedIndent\(treeProps\.depth, 18, 6, 8\)/)
  assert.match(panelSource, /compressedIndent\(rowDepth\(row\), 14, 6, 10\)/)
  assert.match(panelSource, /\.tree-node-row\s*\{[^}]*min-width:\s*max-content/)
  assert.doesNotMatch(panelSource, /\.tree-children\s*\{[^}]*margin-left:\s*18px/)
})
