import assert from 'node:assert/strict'
import test from 'node:test'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

import zhCN from '../src/locales/zh-CN'
import en from '../src/locales/en'

/*
 * vue-i18n's t() accepts any string at the type level (see
 * src/i18n/schema.d.ts), so a missing key is a runtime warn + raw-key render,
 * not a compile error. This test is the compile error: it statically extracts
 * every `t('...')` / `$t('...')` / `tm('...')` key reference from src/ and
 * checks each one against both bundles.
 *
 * Dynamically composed keys (template literals like t(`recording.state.${s}`))
 * cannot be extracted this way; those stay covered by the zh-CN/en parity
 * assertion below, which at least guarantees both languages fail identically.
 */

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

const zhKeys = keysOf(zhCN)
const enKeys = keysOf(en)
const used = referencedKeys()

test('every statically referenced i18n key exists in the zh-CN bundle', () => {
  const missing = [...used].filter((key) => !zhKeys.has(key)).sort()
  assert.deepEqual(missing, [])
})

test('every statically referenced i18n key exists in the en bundle', () => {
  const missing = [...used].filter((key) => !enKeys.has(key)).sort()
  assert.deepEqual(missing, [])
})

test('zh-CN and en define the exact same key set', () => {
  assert.deepEqual([...enKeys].filter((key) => !zhKeys.has(key)).sort(), [])
  assert.deepEqual([...zhKeys].filter((key) => !enKeys.has(key)).sort(), [])
})
