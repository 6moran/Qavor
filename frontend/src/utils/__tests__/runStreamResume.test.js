import assert from 'node:assert/strict'

import {
  compareRunSeq,
  hasOngoingRunChunks,
  normalizeRunSeq,
  resolveRunResumeAfterSeq
} from '../runStreamResume.js'

const run = () => {
  assert.equal(normalizeRunSeq(null), '0-0')
  assert.equal(normalizeRunSeq('bad'), '0-0')
  assert.equal(normalizeRunSeq('1700000000000-2'), '1700000000000-2')

  assert.equal(compareRunSeq('1700000000001-0', '1700000000000-9'), 1)
  assert.equal(compareRunSeq('1700000000000-1', '1700000000000-9'), -1)
  assert.equal(compareRunSeq('1700000000000-1', '1700000000000-1'), 0)

  const emptyThreadState = {
    activeRunId: 'run-1',
    runLastSeq: '1700000000005-0',
    onGoingConv: { msgChunks: {} }
  }
  const activeThreadState = {
    activeRunId: 'run-1',
    runLastSeq: '1700000000005-0',
    onGoingConv: { msgChunks: { 'msg-1': [{ id: 'msg-1', content: '已渲染' }] } }
  }

  assert.equal(hasOngoingRunChunks(emptyThreadState), false)
  assert.equal(hasOngoingRunChunks(activeThreadState), true)

  assert.equal(
    resolveRunResumeAfterSeq({
      snapshot: { run_id: 'run-1', last_seq: '1700000000010-0', client_id: 'client-a' },
      threadState: emptyThreadState
    }),
    '0-0'
  )
  assert.equal(
    resolveRunResumeAfterSeq({
      snapshot: { run_id: 'run-1', last_seq: '1700000000010-0', client_id: 'client-b' },
      threadState: activeThreadState
    }),
    '1700000000005-0'
  )
  assert.equal(
    resolveRunResumeAfterSeq({
      snapshot: { run_id: 'run-2', last_seq: '1700000000010-0', client_id: 'client-a' },
      threadState: activeThreadState
    }),
    '0-0'
  )

  console.log('runStreamResume: all assertions passed')
}

run()
