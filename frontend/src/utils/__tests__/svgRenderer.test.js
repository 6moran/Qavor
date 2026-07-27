import assert from 'node:assert/strict'
import { renderSvgBlocks } from '../svgRenderer.js'

const run = () => {
  // 1. Basic backtick fence
  {
    const result = renderSvgBlocks('before\n```svg\n<svg><circle/></svg>\n```\nafter')
    assert.ok(result.includes('svg-inline-render'), 'Should contain svg-inline-render wrapper')
    assert.ok(result.includes('<svg>'), 'Should contain SVG tag')
    assert.ok(!result.includes('```svg'), 'Should NOT contain raw fence marker')
    assert.ok(result.includes('before'), 'Should preserve content before block')
    assert.ok(result.includes('after'), 'Should preserve content after block')
    // 按钮存在
    assert.ok(result.includes('svg-actions'), 'Should contain svg-actions wrapper')
    assert.ok(result.includes('svg-copy-btn'), 'Should contain copy button')
    assert.ok(result.includes('svg-png-btn'), 'Should contain PNG button')
    assert.ok(result.includes('复制 SVG'), 'Should contain copy text')
    assert.ok(result.includes('复制为 PNG'), 'Should contain PNG text')
    console.log('T1 Basic backtick fence: PASS')
  }

  // 2. Tilde fence
  {
    const result = renderSvgBlocks('~~~svg\n<svg><rect/></svg>\n~~~')
    assert.ok(result.includes('svg-inline-render'), 'Tilde fence should be converted')
    assert.ok(result.includes('<rect/>'), 'Should contain SVG content')
    console.log('T2 Tilde fence: PASS')
  }

  // 3. SVG with blank lines (compressed to single line)
  {
    const result = renderSvgBlocks(
      '```svg\n<svg>\n<defs>\n<linearGradient id="g">\n<stop offset="0%"/>\n\n<stop offset="100%"/>\n</linearGradient>\n</defs>\n</svg>\n```'
    )
    const lines = result.split('\n')
    assert.equal(lines.length, 1, 'Should be compressed to single line')
    assert.ok(result.includes('svg-inline-render'), 'Should contain wrapper')
    assert.ok(
      result.includes('<stop offset="0%"/><stop offset="100%"/>'),
      'Blank lines should be removed between tags'
    )
    assert.ok(result.includes('svg-copy-btn'), 'Buttons should be inside single-line output')
    console.log('T3 Blank lines compressed: PASS')
  }

  // 4. Case insensitive (SVG / Svg / sVG)
  {
    const result = renderSvgBlocks('```SVG\n<svg/>\n```')
    assert.ok(result.includes('svg-inline-render'), 'Uppercase SVG should be matched')
    console.log('T4 Case insensitive: PASS')
  }

  // 5. Incomplete block (streaming safety)
  {
    const result = renderSvgBlocks('before\n```svg\n<svg>')
    assert.ok(result.includes('```svg'), 'Incomplete block should keep raw fence')
    assert.ok(!result.includes('svg-inline-render'), 'Incomplete block should NOT be converted')
    assert.ok(result.includes('before'), 'Should preserve content before')
    console.log('T5 Incomplete block (streaming safety): PASS')
  }

  // 6. Non-SVG code block unchanged
  {
    const result = renderSvgBlocks('```python\nprint(1)\n```')
    assert.ok(result.includes('```python'), 'Python fence should be preserved')
    assert.ok(result.includes('```'), 'Closing fence should be preserved')
    assert.ok(!result.includes('svg-inline-render'), 'Non-SVG block should NOT be converted')
    console.log('T6 Non-SVG code block unchanged: PASS')
  }

  // 7. Multiple consecutive SVG blocks
  {
    const result = renderSvgBlocks('```svg\n<svg id="1"/>\n```\ntext\n```svg\n<svg id="2"/>\n```')
    const matches = result.match(/svg-inline-render/g)
    assert.equal(matches ? matches.length : 0, 2, 'Should convert both SVG blocks')
    const btnMatches = result.match(/svg-copy-btn/g)
    assert.equal(btnMatches ? btnMatches.length : 0, 2, 'Each SVG block should have buttons')
    assert.ok(result.includes('text'), 'Should preserve text between blocks')
    console.log('T7 Multiple SVG blocks: PASS')
  }

  // 8. Empty SVG code block
  {
    const result = renderSvgBlocks('```svg\n```')
    assert.ok(result.includes('svg-inline-render'), 'Empty SVG block should still be converted')
    console.log('T8 Empty SVG block: PASS')
  }

  // 9. No SVG blocks (identity transformation)
  {
    const result = renderSvgBlocks('hello world\n\nsome text')
    assert.equal(result, 'hello world\n\nsome text', 'Should be identity when no SVG blocks')
    console.log('T9 Identity (no SVG blocks): PASS')
  }

  // 10. SVG content with backticks inside
  {
    const result = renderSvgBlocks('```svg\n<svg><title>`code`</title></svg>\n```')
    assert.ok(result.includes('svg-inline-render'), 'Should convert SVG with backticks in content')
    assert.ok(result.includes('`code`'), 'Should preserve backticks inside SVG content')
    console.log('T10 Backticks in SVG content: PASS')
  }

  // 11. Fence with attributes on opening line
  {
    const result = renderSvgBlocks('```svg id="mySvg"\n<svg/>\n```')
    assert.ok(result.includes('svg-inline-render'), 'Fence with attributes should be converted')
    console.log('T11 Fence with attributes: PASS')
  }

  // 12. SVG block at start of content
  {
    const result = renderSvgBlocks('```svg\n<svg/>\n```\n\nsome text after')
    assert.ok(result.includes('svg-inline-render'), 'SVG at start should be converted')
    assert.ok(result.includes('some text after'), 'Should preserve text after')
    console.log('T12 SVG at start: PASS')
  }

  // 13. SVG block at end of content
  {
    const result = renderSvgBlocks('some text before\n\n```svg\n<svg/>\n```')
    assert.ok(result.includes('svg-inline-render'), 'SVG at end should be converted')
    assert.ok(result.includes('some text before'), 'Should preserve text before')
    console.log('T13 SVG at end: PASS')
  }

  // 14. Multiple non-consecutive SVG blocks with other content
  {
    const result = renderSvgBlocks(
      '# Title\n\n```svg\n<svg id="a"/>\n```\n\nSome text\n\n```svg\n<svg id="b"/>\n```\n\n# End'
    )
    const matches = result.match(/svg-inline-render/g)
    assert.equal(matches ? matches.length : 0, 2, 'Should convert both non-consecutive SVG blocks')
    assert.ok(result.includes('# Title'), 'Should preserve markdown headings')
    assert.ok(result.includes('Some text'), 'Should preserve text between blocks')
    assert.ok(result.includes('# End'), 'Should preserve trailing content')
    console.log('T14 Non-consecutive SVG blocks: PASS')
  }

  // 15. SVG content with HTML comments
  {
    const result = renderSvgBlocks('```svg\n<svg><!-- comment --><circle/></svg>\n```')
    assert.ok(result.includes('svg-inline-render'), 'SVG with HTML comments should be converted')
    assert.ok(result.includes('svg-copy-btn'), 'Buttons should be present')
    assert.ok(result.includes('<!-- comment -->'), 'Should preserve HTML comments')
    console.log('T15 SVG with HTML comments: PASS')
  }

  // 16. Action buttons are present in complete SVG blocks
  {
    const result = renderSvgBlocks('```svg\n<svg viewBox="0 0 100 50"><circle/></svg>\n```')
    assert.ok(result.includes('svg-actions'), 'Should contain svg-actions wrapper')
    assert.ok(result.includes('svg-copy-btn'), 'Should contain copy button')
    assert.ok(result.includes('svg-png-btn'), 'Should contain PNG button')
    assert.ok(result.includes('type="button"'), 'Buttons should have type="button"')
    assert.ok(result.includes('复制 SVG'), 'Copy button should have Chinese label')
    assert.ok(result.includes('复制为 PNG'), 'PNG button should have Chinese label')
    // 按钮在 svg 之前（渲染层在上方）
    const actionsIdx = result.indexOf('svg-actions')
    const svgIdx = result.indexOf('<svg ')
    assert.ok(actionsIdx < svgIdx, 'Buttons wrapper should appear before SVG element')
    console.log('T16 Action buttons structure: PASS')
  }

  console.log('\nAll 16 tests passed!')
}

run()
