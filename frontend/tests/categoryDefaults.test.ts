import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import {
  builtInCategoryDetails,
  builtInCategoryKeys,
  categoryKey,
} from '../src/lib/categoryLabel'
import en from '../src/locales/en'
import zhCN from '../src/locales/zh-CN'

/*
 * The category wizard localizes a starter category only while its row still
 * holds the exact text the v12 migration seeded, so the frontend keeps its own
 * copy of that English text (src/lib/categoryLabel.ts). This test guards the
 * duplication: it reads the seed out of internal/storage/migrate.go and fails
 * when a string changes on one side only, when a bundle is missing a key, or
 * when a "translation" is still the English default.
 */

// run-unit-tests.mjs runs with frontend/ as cwd (see i18nKeys.test.ts).
const migrateSource = readFileSync(
  join(process.cwd(), '..', 'internal', 'storage', 'migrate.go'),
  'utf8',
)

// Only the rows literal of seedStarterCategories, so an unrelated UUID
// elsewhere in the migration cannot join the fixture.
function starterSeedSource(): string {
  const start = migrateSource.indexOf('func seedStarterCategories')
  const end = migrateSource.indexOf('for _, r := range rows', start)
  return start < 0 || end < 0 ? '' : migrateSource.slice(start, end)
}

const seedPattern =
  /\{\s*"(00000000-0000-4000-8000-0000000000\d\d)",\s*"([^"]+)",\s*"#[0-9A-Fa-f]{6}",\s*"([^"]+)"/g

const seeded = [...starterSeedSource().matchAll(seedPattern)].map((match) => ({
  name: match[2] ?? '',
  details: match[3] ?? '',
}))

const seededByKey = new Map(seeded.map((row) => [categoryKey(row.name), row.details]))

function localizedDetails(locale: 'zh-CN' | 'en', key: string): unknown {
  const group: Record<string, unknown> =
    locale === 'en' ? en.timeline.categoryDetails : zhCN.timeline.categoryDetails
  return group[key]
}

test('parses the six starter categories out of the migration seed', () => {
  // The parse is part of the fixture: a regex that silently stops matching
  // would make every assertion below vacuous.
  assert.deepEqual(
    seeded.map((row) => row.name),
    ['Focus Work', 'Communication', 'Learning', 'Research', 'Distraction', 'Personal'],
    'seedStarterCategories no longer parses — fix this test, or the expectations below prove nothing',
  )
})

test('the frontend default-copy table is exactly the seeded copy', () => {
  assert.deepEqual({ ...builtInCategoryDetails }, Object.fromEntries(seededByKey))
})

test('every seeded default has a localized label and description in both bundles', () => {
  for (const [key, english] of seededByKey) {
    const translationKey = builtInCategoryKeys[key]
    assert.ok(translationKey !== undefined, `no label key for "${key}"`)

    const zh = localizedDetails('zh-CN', translationKey)
    assert.equal(
      typeof zh,
      'string',
      `zh-CN timeline.categoryDetails.${translationKey} is missing`,
    )
    assert.notEqual(zh, '', `zh-CN timeline.categoryDetails.${translationKey} is empty`)
    assert.notEqual(
      zh,
      english,
      `zh-CN timeline.categoryDetails.${translationKey} is still the English default`,
    )

    assert.equal(
      localizedDetails('en', translationKey),
      english,
      `en timeline.categoryDetails.${translationKey} differs from the seed`,
    )
  }
})
