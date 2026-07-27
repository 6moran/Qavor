import assert from 'node:assert/strict'

import {
  HTML_PREVIEW_HEIGHT,
  HTML_PREVIEW_MAX_HEIGHT,
  HTML_PREVIEW_MIN_HEIGHT,
  HTML_PREVIEW_WIDTH,
  renderHtmlPreviewBlocks
} from '../htmlPreviewRenderer.js'

const identity = (html) => html

const countMatches = (value, pattern) => value.match(pattern)?.length || 0

const run = () => {
  assert.equal(HTML_PREVIEW_WIDTH, 800)
  assert.equal(HTML_PREVIEW_HEIGHT, 360)
  assert.equal(HTML_PREVIEW_MIN_HEIGHT, 1)
  assert.equal(HTML_PREVIEW_MAX_HEIGHT, 700)

  {
    const result = renderHtmlPreviewBlocks(
      'before\n```html:preview\n<div>Hello</div>\n```\nafter',
      { sanitizeHtml: identity }
    )

    assert.ok(result.includes('html-preview-render'))
    assert.ok(result.includes('--html-preview-width: 800px'))
    assert.ok(result.includes('--html-preview-height: 360px'))
    assert.ok(result.includes('--html-preview-min-height: 1px'))
    assert.ok(result.includes('--html-preview-max-height: 700px'))
    assert.ok(!result.includes('html-preview-header'))
    assert.ok(!result.includes('HTML 预览'))
    assert.ok(result.includes('html-preview-frame-slot'))
    assert.ok(result.includes('html-preview-srcdoc'))
    assert.ok(result.includes('&lt;div&gt;Hello&lt;/div&gt;'))
    assert.ok(!result.includes('<iframe'))
    assert.ok(!result.includes('```html:preview'))
    assert.ok(result.includes('before'))
    assert.ok(result.includes('after'))
  }

  {
    const result = renderHtmlPreviewBlocks('```html\n<div>Hello</div>\n```', {
      sanitizeHtml: identity
    })

    assert.ok(result.includes('```html'))
    assert.ok(!result.includes('html-preview-render'))
  }

  {
    const result = renderHtmlPreviewBlocks('before\n```html:preview\n<div>', {
      sanitizeHtml: identity
    })

    assert.ok(result.includes('html-preview-render'))
    assert.ok(result.includes('html-preview-loading-slot'))
    assert.ok(result.includes('html-preview-loading-canvas'))
    assert.ok(result.includes('html-preview-skeleton-title'))
    assert.ok(result.includes('html-preview-skeleton-card'))
    assert.ok(result.includes('HTML 预览加载中'))
    assert.ok(!result.includes('```html:preview'))
    assert.ok(!result.includes('<div>'))
    assert.ok(!result.includes('html-preview-frame-slot'))
  }

  {
    const result = renderHtmlPreviewBlocks('before\n```html:pre\n<div>', {
      sanitizeHtml: identity
    })

    assert.ok(result.includes('html-preview-render'))
    assert.ok(result.includes('html-preview-loading-slot'))
    assert.ok(!result.includes('```html:pre'))
    assert.ok(!result.includes('<div>'))
  }

  {
    const result = renderHtmlPreviewBlocks('```html:pre\n<div>Keep code</div>\n```', {
      sanitizeHtml: identity
    })

    assert.ok(result.includes('```html:pre'))
    assert.ok(result.includes('<div>Keep code</div>'))
    assert.ok(!result.includes('html-preview-render'))
  }

  {
    const result = renderHtmlPreviewBlocks(
      '```html:preview\n<section>A</section>\n```\ntext\n```html:preview\n<section>B</section>\n```',
      { sanitizeHtml: identity }
    )

    assert.equal(countMatches(result, /html-preview-render/g), 2)
    assert.equal(countMatches(result, /html-preview-frame-slot/g), 2)
    assert.ok(result.includes('text'))
  }

  {
    const result = renderHtmlPreviewBlocks('~~~HTML:PREVIEW\n<div>Case</div>\n~~~', {
      sanitizeHtml: identity
    })

    assert.ok(result.includes('html-preview-render'))
    assert.ok(result.includes('&lt;div&gt;Case&lt;/div&gt;'))
  }

  {
    const result = renderHtmlPreviewBlocks(
      '```html:preview\n<div title="`code`">\n\n<span>Line</span>\n</div>\n```',
      { sanitizeHtml: identity }
    )

    assert.ok(result.includes('html-preview-render'))
    assert.ok(result.includes('`code`'))
    assert.ok(result.includes('&#10;&#10;'))
    assert.ok(result.includes('&lt;span&gt;Line&lt;/span&gt;'))
  }

  {
    const result = renderHtmlPreviewBlocks(
      '```html:preview\n<!doctype html>\n<html><head><style>.card{display:grid;color:red}</style></head><body><div class="card">Styled</div></body></html>\n```',
      { sanitizeHtml: identity }
    )

    assert.ok(result.includes('&lt;style&gt;.card{display:grid;color:red}&lt;/style&gt;'))
    assert.ok(result.includes('&lt;div class=&quot;card&quot;&gt;Styled&lt;/div&gt;'))
  }

  {
    const result = renderHtmlPreviewBlocks(
      '```html:preview\n<div>safe</div><script>alert(1)</script>\n```',
      { sanitizeHtml: () => '<div>safe</div>' }
    )

    assert.ok(result.includes('&lt;div&gt;safe&lt;/div&gt;'))
    assert.ok(!result.includes('alert(1)'))
  }
}

run()
