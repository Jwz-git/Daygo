import assert from 'node:assert/strict'
import test from 'node:test'
import { createI18n } from 'vue-i18n'
import { SUPPORTED_LOCALES } from '../src/i18n/locales'
import { loadLocale } from '../src/locales'

test('native pause copy keeps a single time placeholder through vue-i18n in every locale', async () => {
  for (const locale of SUPPORTED_LOCALES) {
    const messages = await loadLocale(locale)
    const i18n = createI18n({ legacy: false, locale, messages: { [locale]: messages } })
    const template = i18n.global.t('recording.menuBar.pausedUntil', { time: '{time}' })
    assert.equal(template.split('{time}').length, 2, locale)
    const displayed = template.replace('{time}', '14:30')
    assert.ok(displayed.includes('14:30'), locale)
    assert.ok(!displayed.includes('{time}'), locale)
  }
})
