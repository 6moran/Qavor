import assert from 'node:assert/strict'

import { makeChildThreadId } from '../subagentThread.js'

const EXPECTED_THREAD_ID = 'subagent_198242794595efedd1850d5263f677c9b628052b1bfca0d5bc77499'

const run = async () => {
  assert.equal(await makeChildThreadId('thread-1', 'researcher', 'call-1'), EXPECTED_THREAD_ID)
  assert.equal(await makeChildThreadId('', 'researcher', 'call-1'), '')

  const originalCrypto = Object.getOwnPropertyDescriptor(globalThis, 'crypto')
  try {
    Object.defineProperty(globalThis, 'crypto', {
      configurable: true,
      value: {}
    })

    assert.equal(await makeChildThreadId('thread-1', 'researcher', 'call-1'), EXPECTED_THREAD_ID)
  } finally {
    if (originalCrypto) {
      Object.defineProperty(globalThis, 'crypto', originalCrypto)
    } else {
      delete globalThis.crypto
    }
  }

  console.log('subagentThread: all assertions passed')
}

await run()
