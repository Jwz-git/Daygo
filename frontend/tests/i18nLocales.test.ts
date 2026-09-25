import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DEFAULT_LANGUAGE,
  DEFAULT_LOCALE,
  FALLBACK_LOCALE,
  SUPPORTED_LOCALES,
  SYSTEM_LANGUAGE,
  normalizeLanguagePreference,
  normalizeLocale,
  resolveLanguage,
  scriptTagOf,
  systemLocale,
  type AppLocale,
  type LocaleScript,
} from '../src/i18n/locales'
import { initialMessages, loadLocale } from '../src/locales'

/*
 * normalizeLocale is the single folding table behind three callers: the system
 * probe, the stored preference, and the Go-side settings.normalizeLanguage that
 * must agree with it. A regression here is silent — the UI renders one language
 * while the settings row stores another — so the table is pinned explicitly.
 */

const FOLDING_CASES: [string, string | null][] = [
  // Chinese: script beats region, and a bare tag is Simplified.
  ['zh', 'zh-CN'],
  ['zh-CN', 'zh-CN'],
  ['zh-Hans', 'zh-CN'],
  ['zh-Hans-CN', 'zh-CN'],
  ['zh-SG', 'zh-CN'],
  ['zh_CN', 'zh-CN'],
  ['zh-Hant', 'zh-Hant'],
  ['zh-Hant-CN', 'zh-Hant'],
  ['zh-TW', 'zh-Hant'],
  ['zh-HK', 'zh-Hant'],
  ['zh-MO', 'zh-Hant'],
  ['zh_tw', 'zh-Hant'],
  // Other shipped languages, region and case insensitively.
  ['en', 'en'],
  ['en-US', 'en'],
  ['EN-gb', 'en'],
  ['ja', 'ja'],
  ['ja-JP', 'ja'],
  ['ko', 'ko'],
  ['ko-KR', 'ko'],
  ['de', 'de'],
  ['de-AT', 'de'],
  ['de-CH', 'de'],
  ['fr', 'fr'],
  ['fr-CA', 'fr'],
  ['es', 'es'],
  ['es-ES', 'es'],
  ['es-MX', 'es'],
  ['es-419', 'es'],
  // Portuguese picks a region for its tag: every region ships pt-BR.
  ['pt', 'pt-BR'],
  ['pt-BR', 'pt-BR'],
  ['pt-PT', 'pt-BR'],
  ['pt-AO', 'pt-BR'],
  // Unshipped languages are not folded onto a neighbour.
  ['frr', null],
  ['it', null],
  ['nl-NL', null],
  ['jv', null],
  ['', null],
  ['   ', null],
  [null, null],
  [undefined, null],
]

for (const [tag, expected] of FOLDING_CASES) {
  test(`normalizeLocale(${JSON.stringify(tag)}) -> ${JSON.stringify(expected)}`, () => {
    assert.equal(normalizeLocale(tag), expected)
  })
}

test('every supported locale is its own normalisation fixpoint', () => {
  for (const locale of SUPPORTED_LOCALES) {
    assert.equal(normalizeLocale(locale), locale)
  }
})

test('every supported locale resolves to a shipped bundle', async () => {
  for (const locale of SUPPORTED_LOCALES) {
    assert.notEqual(await loadLocale(locale), undefined, `no bundle for ${locale}`)
  }
})

/*
 * Every locale but the default is a lazy chunk. Re-adding one to the eager map
 * would put that language's copy in front of the first paint for everyone, which
 * is precisely the cost lazy loading exists to avoid — so it is a deliberate
 * change, not a quiet one.
 */
test('only the default locale is in the initial chunk', () => {
  assert.deepEqual(Object.keys(initialMessages), [DEFAULT_LOCALE])
})

/*
 * The font swap in styles/fonts.css reads this tag to pick a CJK fallback face,
 * so a locale mapped to the wrong script renders in the wrong glyph set. The
 * assertion is against this table rather than against uniqueness: the Latin
 * locales deliberately share 'latn'. Adding a locale without giving it a script
 * here fails the key-set comparison below.
 */
const EXPECTED_SCRIPTS: Record<AppLocale, LocaleScript> = {
  'zh-CN': 'hans',
  'zh-Hant': 'hant',
  en: 'latn',
  ja: 'jpan',
  ko: 'kore',
  de: 'latn',
  fr: 'latn',
  es: 'latn',
  'pt-BR': 'latn',
}

test('every supported locale declares its writing system', () => {
  assert.deepEqual([...SUPPORTED_LOCALES].sort(), Object.keys(EXPECTED_SCRIPTS).sort())
  for (const locale of SUPPORTED_LOCALES) {
    assert.equal(scriptTagOf(locale), EXPECTED_SCRIPTS[locale], `script for ${locale}`)
  }
})

test('normalizeLanguagePreference keeps the empty sentinel and folds tags', () => {
  assert.equal(normalizeLanguagePreference(SYSTEM_LANGUAGE), SYSTEM_LANGUAGE)
  assert.equal(normalizeLanguagePreference('zh-TW'), 'zh-Hant')
  assert.equal(normalizeLanguagePreference('ja-JP'), 'ja')
  assert.equal(normalizeLanguagePreference('pt-PT'), 'pt-BR')
  // An unknown tag is null, not the sentinel: the caller must be able to tell
  // "unreadable preference" from "follow the system" and take the default.
  assert.equal(normalizeLanguagePreference('it'), null)
  assert.equal(normalizeLanguagePreference(42), null)
  assert.equal(normalizeLanguagePreference(null), null)
})

test('resolveLanguage maps the sentinel through the system probe', () => {
  assert.equal(resolveLanguage('ja'), 'ja')
  const original = Object.getOwnPropertyDescriptor(globalThis, 'navigator')
  try {
    Object.defineProperty(globalThis, 'navigator', {
      value: { languages: ['ko-KR', 'en-US'], language: 'ko-KR' },
      configurable: true,
    })
    assert.equal(resolveLanguage(SYSTEM_LANGUAGE), 'ko')
    assert.equal(systemLocale(), 'ko')

    // The first entry that ships wins, not the first entry overall.
    Object.defineProperty(globalThis, 'navigator', {
      value: { languages: ['it-IT', 'fr-CA'], language: 'it-IT' },
      configurable: true,
    })
    assert.equal(systemLocale(), 'fr')

    Object.defineProperty(globalThis, 'navigator', {
      value: { languages: ['it-IT'], language: 'it-IT' },
      configurable: true,
    })
    // Nothing shipped matches, so the branded default wins over English.
    assert.equal(systemLocale(), DEFAULT_LOCALE)
  } finally {
    if (original) Object.defineProperty(globalThis, 'navigator', original)
    else Reflect.deleteProperty(globalThis, 'navigator')
  }
})

test('the branded defaults stay Chinese with an English fallback', () => {
  assert.equal(DEFAULT_LOCALE, 'zh-CN')
  assert.equal(DEFAULT_LANGUAGE, DEFAULT_LOCALE)
  assert.equal(FALLBACK_LOCALE, 'en')
})
