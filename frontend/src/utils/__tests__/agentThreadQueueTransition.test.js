import assert from 'node:assert/strict'
import test from 'node:test'

import { useAgentThreadState } from '../../composables/useAgentThreadState.js'

test('starting the next queued run preserves remaining request streams', () => {
  const chatState = { threadStates: {} }
  const { getThreadState, resetOnGoingConv } = useAgentThreadState({
    chatState,
    getCurrentThreadId: () => 'thread-1'
  })
  const threadState = getThreadState('thread-1')
  let runAborted = false
  let requestAborted = false
  threadState.runStreamAbortController = { abort: () => (runAborted = true) }
  threadState.requestStreams = {
    'request-c': { controller: { abort: () => (requestAborted = true) } }
  }
  threadState.onGoingConv.msgChunks['request-a'] = [{ content: 'A reply' }]

  resetOnGoingConv('thread-1', { preserveRequestStreams: true })

  assert.equal(runAborted, true)
  assert.equal(requestAborted, false)
  assert.deepEqual(Object.keys(threadState.requestStreams), ['request-c'])
  assert.deepEqual(threadState.onGoingConv.msgChunks, {})
})
