import assert from 'node:assert/strict'
import test from 'node:test'

import zhCN from '../src/locales/zh-CN'
import en from '../src/locales/en'
import { failurePresentation } from '../src/views/Timeline/failurePresentation'

function translation(bundle: object, key: string): unknown {
  return key.split('.').reduce<unknown>((node, part) => {
    if (typeof node !== 'object' || node === null) return undefined
    return (node as Record<string, unknown>)[part]
  }, bundle)
}

test('provider failures remain explicit while application failures do not blame the provider', () => {
  for (const kind of ['auth', 'rate_limited', 'network', 'invalid_request', 'invalid_output']) {
    const display = failurePresentation(kind)
    assert.equal(display.source, 'provider', kind)
    assert.equal(display.titleKey, 'timeline.failure.providerTitle', kind)
  }
  assert.equal(failurePresentation('no_provider').source, 'configuration')
  assert.equal(failurePresentation('internal').source, 'application')
  assert.equal(failurePresentation('media').source, 'application')
})

test('each provider failure has a specific localized reason key', () => {
  const reasons = new Set(
    ['auth', 'rate_limited', 'network', 'invalid_request', 'invalid_output']
      .map((kind) => failurePresentation(kind).reasonKey),
  )
  assert.equal(reasons.size, 5)
})

test('all failure presentation keys resolve in both languages', () => {
  for (const kind of ['auth', 'rate_limited', 'network', 'invalid_request', 'invalid_output', 'no_provider', 'internal', 'unknown']) {
    const display = failurePresentation(kind)
    for (const key of [display.titleKey, display.reasonKey, display.actionKey]) {
      assert.equal(typeof translation(zhCN, key), 'string', `zh-CN ${key}`)
      assert.equal(typeof translation(en, key), 'string', `en ${key}`)
    }
  }
})
