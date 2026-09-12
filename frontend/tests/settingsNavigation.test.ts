import assert from 'node:assert/strict'
import test from 'node:test'

import { settingsSectionFromQuery } from '../src/views/Settings/navigation'

test('settings deep links accept known sections and fall back to general', () => {
  assert.equal(settingsSectionFromQuery('ai'), 'ai')
  assert.equal(settingsSectionFromQuery('recording'), 'recording')
  assert.equal(settingsSectionFromQuery('providers'), 'general')
  assert.equal(settingsSectionFromQuery(['ai']), 'general')
  assert.equal(settingsSectionFromQuery(undefined), 'general')
})
