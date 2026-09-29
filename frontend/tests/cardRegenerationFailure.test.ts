import assert from 'node:assert/strict'
import test from 'node:test'

import zhCN from '../src/locales/zh-CN'
import en from '../src/locales/en'
import { cardRegenerationFailureKey } from '../src/views/Timeline/cardRegenerationFailure'

test('card regeneration errors use the binding code and never expose diagnostic text', () => {
  assert.equal(
    cardRegenerationFailureKey(new Error('daygo:provider_failed: secret diagnostic')),
    'timeline.reprocess.error.provider_failed',
  )
  assert.equal(
    cardRegenerationFailureKey('daygo:conflict: another batch is running'),
    'timeline.reprocess.error.conflict',
  )
  assert.equal(
    cardRegenerationFailureKey({ message: 'daygo:provider_not_configured: none' }),
    'timeline.reprocess.error.provider_not_configured',
  )
  assert.equal(cardRegenerationFailureKey('arbitrary provider text'), 'timeline.reprocess.error.application')
})

test('every card regeneration reason has localized copy', () => {
  for (const code of [
    'provider_not_configured', 'provider_failed', 'conflict', 'invalid_argument',
    'not_capture_owner', 'not_found', 'canceled', 'application',
  ] as const) {
    assert.equal(typeof zhCN.timeline.reprocess.error[code], 'string', `zh-CN ${code}`)
    assert.equal(typeof en.timeline.reprocess.error[code], 'string', `en ${code}`)
  }
})
