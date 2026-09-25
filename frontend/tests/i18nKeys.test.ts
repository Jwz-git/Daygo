import assert from 'node:assert/strict'
import test from 'node:test'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

import { SUPPORTED_LOCALES, type AppLocale } from '../src/i18n/locales'
import { initialMessages, loadLocale, type LocaleSchema } from '../src/locales'

/*
 * vue-i18n's t() accepts any string at the type level (see
 * src/i18n/schema.d.ts), so a missing key is a runtime warn + raw-key render,
 * not a compile error. This test is the compile error: it statically extracts
 * every `t('...')` / `$t('...')` / `tm('...')` key reference from src/ and
 * checks each one against every shipped bundle.
 *
 * The loader map's Record<AppLocale, () => Promise<{default: LocaleSchema}>>
 * annotation in src/locales/index.ts already fails vue-tsc when a bundle drifts
 * from zh-CN, so the parity assertions here are a backstop for that annotation
 * being weakened rather than the primary gate. The referenced-key check is not
 * redundant: it catches a key that exists in no bundle at all, which types
 * alone cannot see.
 *
 * Dynamically composed keys (template literals like t(`recording.state.${s}`))
 * cannot be extracted this way; those stay covered by the parity assertion
 * below, which at least guarantees every language fails identically.
 */

/*
 * Loaded through the real loader map rather than imported file by file, so this
 * file also proves the lazy chunks resolve. An import path that points at a
 * missing file fails here instead of in the packaged app, where the only
 * symptom would be one language rendering raw keys.
 */
const bundles = new Map<AppLocale, LocaleSchema>()
for (const locale of SUPPORTED_LOCALES) {
  bundles.set(locale, await loadLocale(locale))
}

function bundle(locale: AppLocale): LocaleSchema {
  const found = bundles.get(locale)
  if (!found) throw new Error(`no bundle loaded for ${locale}`)
  return found
}

// run-unit-tests.mjs bundles tests into a temp dir before executing, so
// import.meta.url points at /tmp. The runner is always invoked with the
// frontend/ directory as cwd; resolve src/ from there.
const srcRoot = join(process.cwd(), 'src')

function collectSourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'locales') continue
      out.push(...collectSourceFiles(full))
    } else if (entry.name.endsWith('.vue') || entry.name.endsWith('.ts')) {
      out.push(full)
    }
  }
  return out
}

const referencePattern = /(?:\$|(?<![\w$]))t(?:m)?\(\s*'([a-zA-Z0-9_.]+)'\s*[,)]/g

function referencedKeys(): Set<string> {
  const keys = new Set<string>()
  for (const file of collectSourceFiles(srcRoot)) {
    const source = readFileSync(file, 'utf8')
    // Strip comments first: doc comments routinely quote t('...') as prose,
    // and those are not references.
    const code = source
      .replace(/\/\*[\s\S]*?\*\//g, ' ')
      .replace(/\/\/[^\n]*/g, ' ')
    for (const match of code.matchAll(referencePattern)) {
      keys.add(match[1])
    }
  }
  return keys
}

function flattenKeys(node: unknown, prefix: string, out: Set<string>): void {
  if (typeof node === 'object' && node !== null) {
    for (const [key, value] of Object.entries(node)) {
      const next = prefix === '' ? key : `${prefix}.${key}`
      // Arrays addressed as a whole via tm('chat.welcome.hints') are
      // referenced at their own level, so a nested array keeps the prefix.
      if (Array.isArray(value)) out.add(next)
      else if (typeof value === 'object' && value !== null) flattenKeys(value, next, out)
      else out.add(next)
    }
  }
}

function keysOf(bundle: unknown): Set<string> {
  const out = new Set<string>()
  flattenKeys(bundle, '', out)
  return out
}

const zhKeys = keysOf(bundle('zh-CN'))
const used = referencedKeys()

const bundlesByLocale = SUPPORTED_LOCALES.map(
  (locale) => [locale, keysOf(bundle(locale))] as const,
)

test('only the default locale is in the initial chunk', () => {
  assert.deepEqual(Object.keys(initialMessages), ['zh-CN'])
})

for (const [locale, keys] of bundlesByLocale) {
  test(`every statically referenced i18n key exists in the ${locale} bundle`, () => {
    const missing = [...used].filter((key) => !keys.has(key)).sort()
    assert.deepEqual(missing, [])
  })

  test(`the ${locale} bundle defines the exact same key set as zh-CN`, () => {
    assert.deepEqual([...keys].filter((key) => !zhKeys.has(key)).sort(), [])
    assert.deepEqual([...zhKeys].filter((key) => !keys.has(key)).sort(), [])
  })
}

/*
 * A bundle value byte-identical to the English one is usually a missed
 * translation, and nothing else in the build catches it — a key that exists but
 * still holds English copy type-checks and renders fine.
 *
 * The exceptions are listed here by *value*, not by key, so this stays short
 * and adding a language does not require touching it. Two kinds:
 * placeholders with no surrounding prose, and words several languages simply
 * share. Stripping placeholders first is what makes short words comparable:
 * "{count} min" reduces to "min", "{current} / {total}" reduces to nothing and
 * is skipped without needing an entry.
 *
 * If this fails, either translate the value or add what remains after stripping
 * the placeholders to SHARED_WORDING — a deliberate decision either way.
 */
const SHARED_WORDING = new Set([
  // Product names and proper nouns.
  'Daygo',
  'MCP CLI',
  'OpenAI Chat Completions',
  'OpenAI Responses',
  'Anthropic',
  'Windows Recorder',
  // Units and abbreviations written the same across the shipped set. Note
  // "{count} min" and "720 pixels" reduce to 'min' and 'pixels'.
  'GB',
  'Go',
  'min',
  'minutes',
  'pixels',
  'images',
  // Ordinary words that coincide across languages.
  'Cause',
  'Chat',
  'Communication',
  'Composition',
  'Distraction',
  'Distractions',
  'Export',
  'Intentions',
  'Journal',
  'Neutral',
  'Notes',
  'Pause',
  'Personal',
  'Plan',
  'Test',
  'Total',
])

/** The prose in a value, with placeholders, digits and punctuation removed. */
function copyOutsidePlaceholders(value: string): string {
  return value
    .replace(/\{\w+\}/g, ' ')
    .replace(/[^\p{L}]+/gu, ' ')
    .trim()
}

function leafEntries(node: unknown, prefix: string, out: Map<string, string>): void {
  if (Array.isArray(node)) {
    node.forEach((item, index) => leafEntries(item, `${prefix}.${index}`, out))
    return
  }
  if (typeof node === 'object' && node !== null) {
    for (const [key, value] of Object.entries(node)) {
      leafEntries(value, prefix === '' ? key : `${prefix}.${key}`, out)
    }
    return
  }
  if (typeof node === 'string') out.set(prefix, node)
}

const enLeaves = new Map<string, string>()
leafEntries(bundle('en'), '', enLeaves)

for (const locale of SUPPORTED_LOCALES) {
  if (locale === 'en') continue

  test(`the ${locale} bundle is not carrying English copy`, () => {
    const leaves = new Map<string, string>()
    leafEntries(bundle(locale), '', leaves)

    const untranslated = [...leaves]
      .filter(([key, value]) => enLeaves.get(key) === value)
      .filter(([, value]) => {
        const prose = copyOutsidePlaceholders(value)
        return prose !== '' && !SHARED_WORDING.has(prose)
      })
      .map(([key, value]) => `${key} = ${JSON.stringify(value)}`)
      .sort()

    assert.deepEqual(untranslated, [])
  })
}
