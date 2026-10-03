import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { SUPPORTED_LOCALES, type AppLocale } from '../src/i18n/locales'
import { loadLocale, type LocaleSchema } from '../src/locales'
import { chatFailureMessage } from '../src/lib/chatFailure'

/*
 * A failed chat turn stores a machine code, not prose, and the bubble renders
 * it as chat.failure.<code>. Nothing else joins the two sides: Go writes a code
 * the TypeScript side has never heard of just as happily as a valid one, and a
 * missing translation renders as a raw key. This test reads the code list out
 * of the Go source and holds every bundle to it, in both directions.
 *
 * run-unit-tests.mjs runs with frontend/ as cwd (see i18nKeys.test.ts).
 */
const chatSource = readFileSync(
  join(process.cwd(), '..', 'internal', 'chat', 'chat.go'),
  'utf8',
)

// Only the failure-code block, so an unrelated `x = "y"` in the same file
// cannot join the code set and make the assertions below vacuous.
function failureCodeBlock(): string {
  const start = chatSource.indexOf('// Terminal failure codes')
  const end = chatSource.indexOf('\n)', start)
  return start < 0 || end < 0 ? '' : chatSource.slice(start, end)
}

const codePattern = /failure[A-Za-z]+\s+= "([a-z_]+)"/g

const goCodes = [...failureCodeBlock().matchAll(codePattern)].map((match) => match[1])

const bundles = new Map<AppLocale, LocaleSchema>()
for (const locale of SUPPORTED_LOCALES) {
  bundles.set(locale, await loadLocale(locale))
}

function bundle(locale: AppLocale): LocaleSchema {
  const found = bundles.get(locale)
  if (!found) throw new Error(`no bundle loaded for ${locale}`)
  return found
}

test('parses the failure codes out of the chat service', () => {
  // The parse is part of the fixture: a regex that silently stopped matching
  // would make every assertion below pass on an empty set.
  assert.ok(
    goCodes.length >= 10,
    `parsed ${goCodes.length} failure codes out of internal/chat/chat.go — fix this test`,
  )
  assert.equal(new Set(goCodes).size, goCodes.length, 'duplicate failure code in internal/chat/chat.go')
})

for (const locale of SUPPORTED_LOCALES) {
  test(`every failure code has a ${locale} message`, () => {
    const failure: Record<string, string> = bundle(locale).chat.failure
    const missing = goCodes.filter((code) => {
      const value = failure[code]
      return typeof value !== 'string' || value.trim() === ''
    })
    assert.deepEqual(missing, [], `${locale} is missing chat.failure.<code> for: ${missing.join(', ')}`)
  })

  test(`the ${locale} failure messages are all reachable from a backend code`, () => {
    const known = new Set(goCodes)
    const unreachable = Object.keys(bundle(locale).chat.failure).filter((code) => !known.has(code))
    assert.deepEqual(
      unreachable,
      [],
      `${locale} translates chat.failure keys the backend can never write: ${unreachable.join(', ')}`,
    )
  })
}

// The resolver itself, against the real bundles: this is what the bubble runs.
function resolverFor(locale: AppLocale) {
  const failure: Record<string, string> = bundle(locale).chat.failure
  return {
    te: (key: string) => Object.hasOwn(failure, key.slice('chat.failure.'.length)),
    t: (key: string) => failure[key.slice('chat.failure.'.length)] ?? key,
  }
}

test('a failed turn resolves to the localized line in every language', () => {
  for (const locale of SUPPORTED_LOCALES) {
    const { te, t } = resolverFor(locale)
    for (const code of goCodes) {
      const text = chatFailureMessage({ role: 'assistant', status: 'failed', errorCode: code }, te, t)
      assert.notEqual(text, '', `${locale}: ${code} resolved to nothing`)
      assert.ok(!text.startsWith('chat.failure.'), `${locale}: ${code} rendered a raw key`)
    }
  }
})

test('an unknown code falls back to the row text rather than a raw key', () => {
  const { te, t } = resolverFor('en')
  const unknown = { role: 'assistant', status: 'failed', errorCode: 'something_newer' }
  assert.equal(chatFailureMessage(unknown, te, t), '')
})

test('only a terminal assistant row has a failure line', () => {
  const { te, t } = resolverFor('en')
  assert.equal(chatFailureMessage({ role: 'assistant', status: 'ok', errorCode: 'connection' }, te, t), '')
  assert.equal(chatFailureMessage({ role: 'assistant', status: '', errorCode: 'connection' }, te, t), '')
  assert.equal(chatFailureMessage({ role: 'user', status: 'failed', errorCode: 'connection' }, te, t), '')
  assert.equal(chatFailureMessage({ role: 'assistant', status: 'failed', errorCode: '' }, te, t), '')
  assert.notEqual(
    chatFailureMessage({ role: 'assistant', status: 'canceled', errorCode: 'canceled' }, te, t),
    '',
  )
})
