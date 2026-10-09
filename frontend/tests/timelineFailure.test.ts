import assert from 'node:assert/strict'
import test from 'node:test'

import zhCN from '../src/locales/zh-CN'
import en from '../src/locales/en'
import zhHant from '../src/locales/zh-Hant'
import ja from '../src/locales/ja'
import ko from '../src/locales/ko'
import de from '../src/locales/de'
import fr from '../src/locales/fr'
import es from '../src/locales/es'
import ptBR from '../src/locales/pt-BR'
import { failurePresentation } from '../src/views/Timeline/failurePresentation'

const providerKinds = ['auth', 'rate_limited', 'network', 'timeout', 'service_unavailable', 'invalid_request', 'invalid_output']
const bundles = { 'zh-CN': zhCN, en, 'zh-Hant': zhHant, ja, ko, de, fr, es, 'pt-BR': ptBR }

function translation(bundle: object, key: string): unknown {
  return key.split('.').reduce<unknown>((node, part) => {
    if (typeof node !== 'object' || node === null) return undefined
    return (node as Record<string, unknown>)[part]
  }, bundle)
}

test('provider failures remain explicit while application failures do not blame the provider', () => {
  for (const kind of providerKinds) {
    const display = failurePresentation(kind)
    assert.equal(display.source, 'provider', kind)
    assert.equal(display.titleKey, kind === 'invalid_output' ? 'timeline.failure.outputTitle' : 'timeline.failure.providerTitle', kind)
  }
  assert.equal(failurePresentation('no_provider').source, 'configuration')
  assert.equal(failurePresentation('internal').source, 'application')
  assert.equal(failurePresentation('media').source, 'application')
})

test('invalid output suggests generation recovery without blaming credentials', () => {
  const display = failurePresentation('invalid_output')
  assert.equal(display.actionKey, 'timeline.failure.outputAction')
  assert.equal(display.reasonKey, 'timeline.failure.reason.invalid_output')
  for (const [locale, bundle] of Object.entries(bundles)) {
    for (const key of ['timeline.failure.outputTitle', 'timeline.failure.outputAction']) {
      assert.equal(typeof translation(bundle, key), 'string', `${locale} ${key}`)
    }
  }
})

test('each provider failure has a specific localized reason key', () => {
  const reasons = new Set(
    providerKinds
      .map((kind) => failurePresentation(kind).reasonKey),
  )
  assert.equal(reasons.size, providerKinds.length)
})

test('all failure presentation keys resolve in all nine languages', () => {
  for (const kind of [...providerKinds, 'no_provider', 'internal', 'unknown']) {
    const display = failurePresentation(kind)
    for (const key of [display.titleKey, display.reasonKey, display.actionKey]) {
      for (const [locale, bundle] of Object.entries(bundles)) {
        assert.equal(typeof translation(bundle, key), 'string', `${locale} ${key}`)
      }
    }
  }
})
