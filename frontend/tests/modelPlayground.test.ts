import assert from 'node:assert/strict'
import test from 'node:test'
import { createPlaygroundSession, imageFileError } from '../src/stores/modelPlaygroundSession'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import ModelReply from '../src/views/ModelPlayground/ModelReply.vue'
import { SUPPORTED_LOCALES } from '../src/i18n/locales'
import { loadLocale } from '../src/locales'

test('dynamic playground status keys exist in every shipped language', async () => {
  for (const locale of SUPPORTED_LOCALES) {
    const messages = (await loadLocale(locale)).modelPlayground
    for (const key of ['unavailable', 'readOnly', 'failed', 'copied', 'copyFailed'] as const) {
      assert.ok(messages[key].length > 0, `${locale}: ${key}`)
    }
  }
})

test('actual model replies remain readable text without active HTML or remote images', async () => {
  const payload = '<script>alert(1)</script><img src="https://example.invalid/pixel">\n![remote](https://example.invalid/pixel)'
  const html = await renderToString(createSSRApp(ModelReply, { text: payload }))
  assert.ok(html.includes('&lt;script&gt;alert(1)&lt;/script&gt;'))
  assert.ok(html.includes('![remote](https://example.invalid/pixel)'))
  assert.ok(!html.includes('<script>'))
  assert.ok(!html.includes('<img'))
})

test('a pending trial cannot send twice, and clearing discards its late result', async () => {
  let calls = 0
  let finish!: (result: { ok: boolean; text: string; model: string; latencyMs: number; errorCode: string }) => void
  const session = createPlaygroundSession(() => {
    calls++
    return new Promise((resolve) => { finish = resolve })
  })
  const request = { providerId: 'p', model: 'm', text: 'fixture', imageType: '', imageBase64: '' }
  const pending = session.send(request)
  await session.send(request)
  assert.equal(calls, 1)
  session.clear()
  finish({ ok: true, text: '<script>fixture</script>', model: 'm', latencyMs: 20, errorCode: '' })
  await pending
  assert.equal(session.result.value, null)
  assert.equal(session.busy.value, false)
})

test('binding failures become safe visible errors and a new request clears the old response', async () => {
  const session = createPlaygroundSession(async () => { throw new Error('private backend detail') })
  await session.send({ providerId: 'p', model: 'm', text: 'fixture', imageType: '', imageBase64: '' })
  assert.equal(session.error.value, 'failed')
  assert.equal(session.result.value, null)
  assert.equal(session.busy.value, false)
})

test('image upload rejects empty, oversized and non-image inputs', () => {
  assert.equal(imageFileError({ type: 'image/png', size: 12 }), '')
  assert.equal(imageFileError({ type: 'image/jpeg', size: 5 * 1024 * 1024 }), '')
  assert.equal(imageFileError({ type: 'image/png', size: 5 * 1024 * 1024 + 1 }), 'imageInvalid')
  assert.equal(imageFileError({ type: 'image/svg+xml', size: 10 }), 'imageInvalid')
  assert.equal(imageFileError({ type: 'image/png', size: 0 }), 'imageInvalid')
})
