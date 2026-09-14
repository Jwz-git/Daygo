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
