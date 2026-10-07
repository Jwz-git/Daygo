import assert from 'node:assert/strict'
import test from 'node:test'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'

import zhCN from '../src/locales/zh-CN'
import PlanBlockForm from '../src/views/Timeline/PlanBlockForm.vue'
import { useCapabilitiesStore } from '../src/stores/capabilities'

test('notification capability is absent until confirmed and retries failed discovery', async () => {
  const previousWindow = globalThis.window
  let fail = true
  let calls = 0
  globalThis.window = { go: { app: { Backend: { GetCapabilities: async () => {
    calls += 1
    if (fail) throw new Error('anonymous failure')
    return { canWrite: true, isCaptureOwner: true, features: ['notifications'], appVersion: 'test', apiRevision: 1 }
  } } } } } as unknown as Window & typeof globalThis
  try {
    setActivePinia(createPinia())
    const store = useCapabilitiesStore()
    assert.equal(store.notificationsAvailable, false)
    await store.load()
    assert.equal(store.notificationsAvailable, false)
    fail = false
    await store.load()
    assert.equal(store.notificationsAvailable, true)
    await store.load()
    assert.equal(calls, 2, 'capabilities are immutable within this process')
  } finally {
    globalThis.window = previousWindow
  }
})

test('plan reminder controls reflect capability without clearing saved intent', async () => {
  for (const available of [false, true]) {
    const app = createSSRApp(PlanBlockForm, {
      initial: { id: 1, start: '09:00', end: '10:00', title: 'Anonymous', notes: '', categoryId: '', remind: true },
      categories: [], pending: false, error: '', notificationsAvailable: available,
    })
    app.use(createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN } }))
    const html = await renderToString(app)
    const checkbox = html.match(/<input[^>]*type="checkbox"[^>]*>/)?.[0]
    assert.ok(checkbox)
    assert.equal(checkbox.includes('disabled'), !available)
    assert.equal(checkbox.includes('checked'), true, 'existing reminder preference is retained')
    assert.equal(html.includes(zhCN.settings.general.notificationsUnavailable), !available)
  }
})
