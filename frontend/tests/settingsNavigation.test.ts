import assert from 'node:assert/strict'
import test from 'node:test'

import { settingsSectionFromQuery } from '../src/views/Settings/navigation'

test('settings deep links accept known sections and fall back to general', () => {
  assert.equal(settingsSectionFromQuery('providers'), 'providers')
  assert.equal(settingsSectionFromQuery('recording'), 'recording')
  assert.equal(settingsSectionFromQuery('general'), 'general')
  assert.equal(settingsSectionFromQuery('storage'), 'storage')
  assert.equal(settingsSectionFromQuery('agentAccess'), 'agentAccess')
  assert.equal(settingsSectionFromQuery('unknown'), 'general')
  assert.equal(settingsSectionFromQuery(['providers']), 'general')
  assert.equal(settingsSectionFromQuery(undefined), 'general')
})

// A settings section that exists but is mounted nowhere is a setting users can
// no longer change while the backend keeps honouring it. That happened to the
// model output language (dropped from SettingsView in 6a7911a) with no test
// noticing; every *Section.vue must be imported by another settings file.
test('every settings section component is mounted', async () => {
  const { readFile, readdir } = await import('node:fs/promises')
  const directory = 'src/views/Settings'
  const files = (await readdir(directory)).filter((name) => name.endsWith('.vue'))
  const sources = new Map(
    await Promise.all(files.map(async (name) => [name, await readFile(`${directory}/${name}`, 'utf8')] as const)),
  )
  for (const name of files.filter((file) => file.endsWith('Section.vue'))) {
    const importers = [...sources].filter(([other, source]) => other !== name && source.includes(`'./${name}'`))
    assert.ok(importers.length > 0, `${name} is not mounted by any settings view`)
  }
})
