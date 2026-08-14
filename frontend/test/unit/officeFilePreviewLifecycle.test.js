import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createServer } from 'vite'

const frontendRoot = fileURLToPath(new URL('../..', import.meta.url))
let viteServer

test.before(async () => {
  viteServer = await createServer({
    root: frontendRoot,
    appType: 'custom',
    logLevel: 'silent',
    server: { middlewareMode: true, hmr: false }
  })
})

test.after(async () => {
  await viteServer?.close()
})

test('OfficeFilePreview initializes its immediate watcher without throwing', async () => {
  const { default: OfficeFilePreview } = await viteServer.ssrLoadModule(
    '/src/components/common/OfficeFilePreview.vue'
  )
  const app = createSSRApp({
    render: () =>
      h(OfficeFilePreview, {
        file: { previewType: 'xlsx', previewUrl: '' },
        filePath: 'example.xlsx'
      })
  })
  app.component('a-spin', { render: () => null })
  app.component('a-select', { render: () => null })

  await assert.doesNotReject(() => renderToString(app))
})

test('preview render gate accepts only the latest render task', async () => {
  const { createPreviewRenderGate } = await import(
    '../../src/utils/office_preview_lifecycle.js'
  )
  const gate = createPreviewRenderGate()

  const firstRun = gate.begin()
  const secondRun = gate.begin()

  assert.equal(gate.isCurrent(firstRun), false)
  assert.equal(gate.isCurrent(secondRun), true)
})

test('preview render gate invalidates the active task during cleanup', async () => {
  const { createPreviewRenderGate } = await import(
    '../../src/utils/office_preview_lifecycle.js'
  )
  const gate = createPreviewRenderGate()
  const activeRun = gate.begin()

  gate.invalidate()

  assert.equal(gate.isCurrent(activeRun), false)
})

test('AgentFilePreview uses the auto-unwrapped Office preview computed in its template', async () => {
  const source = await readFile(
    new URL('../../src/components/AgentFilePreview.vue', import.meta.url),
    'utf8'
  )

  assert.doesNotMatch(source, /isOfficePreviewType\.value/)
})

test('OfficeFilePreview keeps renderer containers mounted while loading', async () => {
  const source = await readFile(
    new URL('../../src/components/common/OfficeFilePreview.vue', import.meta.url),
    'utf8'
  )
  const template = source.slice(0, source.indexOf('</template>'))

  assert.match(template, /<div v-if="loading"[^>]*class="office-state office-loading"/)
  assert.match(template, /<div v-if="error"[^>]*class="office-state office-error"/)
})
