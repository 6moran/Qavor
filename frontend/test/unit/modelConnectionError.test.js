import assert from 'node:assert/strict'
import test from 'node:test'
import { formatModelTestError } from '../../src/utils/modelConfig.js'

test('extracts friendly message and detail from error response data', () => {
  const error = new Error('openai: error, status code: 402, message: insufficient_quota')
  error.response = {
    status: 500,
    data: {
      code: 4003,
      message: '账户余额不足或额度已用尽，请前往服务商控制台充值或检查配额',
      detail: 'openai: error, status code: 402, message: insufficient_quota'
    }
  }
  const result = formatModelTestError(error)
  assert.equal(result.message, '账户余额不足或额度已用尽，请前往服务商控制台充值或检查配额')
  assert.equal(result.detail, 'openai: error, status code: 402, message: insufficient_quota')
})

test('falls back to error.message when response data is missing', () => {
  const error = new Error('网络异常')
  const result = formatModelTestError(error)
  assert.equal(result.message, '网络异常')
  assert.equal(result.detail, '')
})

test('uses default message when nothing is available', () => {
  const result = formatModelTestError({})
  assert.equal(result.message, '连接测试失败')
  assert.equal(result.detail, '')
})
