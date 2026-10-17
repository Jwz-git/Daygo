import assert from 'node:assert/strict'
import test from 'node:test'

import { CATEGORY_ARCHETYPES } from '../src/api/dto'
import { SUPPORTED_LOCALES } from '../src/i18n/locales'
import {
  PROFESSION_PRESETS,
  PROFESSION_PRESET_IDS,
  presetRows,
  professionPreset,
} from '../src/lib/categoryPresets'
import { loadLocale } from '../src/locales'

/*
 * First-run presets follow Dayflow's OnboardingCategoryPreset. The English
 * names below are copied from that source (Models/TimelineCategory.swift) as
 * the fixture, so a drift from the reference fails here. Every locale must
 * resolve every key to real copy: a preset is stored as plain text, and a
 * missing key would become a category literally named after the key.
 */

const DAYFLOW_NAMES: Record<string, string[]> = {
  'software-engineer': ['Coding / Debugging', 'Code Review', 'Research', 'Communication', 'Distraction', 'Personal'],
  'founder-executive': ['Engineering / Product', 'Research & Strategy', 'Data & Insights', 'Communication', 'Distraction', 'Personal'],
  designer: ['Design', 'Research', 'Communication', 'Distraction', 'Personal'],
  student: ['Studying', 'Assignments', 'Communication', 'Distraction', 'Personal'],
  'product-manager': ['Specs & Planning', 'Research & Analysis', 'Communication', 'Distraction', 'Personal'],
  'data-scientist': ['Analysis & Modeling', 'Data Engineering', 'Research', 'Communication', 'Distraction', 'Personal'],
  other: ['Work', 'Communication', 'Distraction', 'Personal'],
}

function lookup(bundle: unknown, key: string): unknown {
  return key.split('.').reduce<unknown>(
    (node, part) => (typeof node === 'object' && node !== null ? (node as Record<string, unknown>)[part] : undefined),
    bundle,
  )
}

function translator(bundle: unknown, locale: string): (key: string) => string {
  return (key) => {
    const value = lookup(bundle, key)
    assert.equal(typeof value, 'string', `${locale} is missing ${key}`)
    return value as string
  }
}

test('presets are Dayflow\'s seven roles, each ending with Distraction and Personal', () => {
  assert.deepEqual(PROFESSION_PRESETS.map((preset) => preset.id), [...PROFESSION_PRESET_IDS])
  assert.deepEqual(Object.keys(DAYFLOW_NAMES), [...PROFESSION_PRESET_IDS])
  for (const preset of PROFESSION_PRESETS) {
    assert.equal(professionPreset(preset.id), preset)
    const keys = preset.categories.map((category) => category.key)
    assert.equal(new Set(keys).size, keys.length, `${preset.id} repeats a key`)
    assert.deepEqual(keys.slice(-2), ['distraction', 'personal'], `${preset.id} tail`)
    for (const category of preset.categories) {
      assert.match(category.colorHex, /^#[0-9A-F]{6}$/, `${preset.id}.${category.key}`)
      assert.ok((CATEGORY_ARCHETYPES as readonly string[]).includes(category.archetype), `${preset.id}.${category.key}`)
    }
  }
})

test('the English rows reproduce Dayflow\'s preset names', async () => {
  const translate = translator(await loadLocale('en'), 'en')
  for (const preset of PROFESSION_PRESETS) {
    assert.deepEqual(presetRows(preset, translate).map((row) => row.name), DAYFLOW_NAMES[preset.id], preset.id)
  }
})

for (const locale of SUPPORTED_LOCALES) {
  test(`${locale}: preset rows resolve to unique copy`, async () => {
    const bundle = await loadLocale(locale)
    const translate = translator(bundle, locale)
    for (const preset of PROFESSION_PRESETS) {
      assert.notEqual(translate(`presets.profession.${preset.nameKey}.name`).trim(), '')
      const rows = presetRows(preset, translate)
      const names = rows.map((row) => row.name.trim().toLocaleLowerCase())
      assert.equal(new Set(names).size, rows.length, `${locale} ${preset.id} has duplicate names`)
      rows.forEach((row, index) => {
        const category = preset.categories[index]
        assert.notEqual(row.name.trim(), '', `${locale} ${preset.id} row ${index} name`)
        assert.notEqual(row.details.trim(), '', `${locale} ${preset.id} row ${index} details`)
        assert.ok(!['system', 'idle'].includes(names[index] ?? ''), `${locale} ${row.name} is a built-in`)
        assert.equal(row.colorHex, category?.colorHex)
        assert.equal(row.archetype, category?.archetype)
      })
    }
    for (const archetype of CATEGORY_ARCHETYPES) translate(`presets.archetype.${archetype}`)
  })
}
