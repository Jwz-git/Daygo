import assert from 'node:assert/strict'
import test from 'node:test'

import {
  createDevelopmentTestDataSelector,
  resolveDevelopmentTestData,
} from '../src/api/developmentFixtures'

test('selects development test data before or after the hash', () => {
  assert.equal(resolveDevelopmentTestData('http://localhost:5173/?testData=on#/timeline', true), true)
  assert.equal(resolveDevelopmentTestData('http://localhost:5173/#/timeline?testData=off', true), false)
  assert.equal(resolveDevelopmentTestData('http://localhost:5173/#/daily?testData=1', true), true)
  assert.equal(resolveDevelopmentTestData('http://localhost:5173/#/weekly?testData=0', true), false)
})

test('keeps the development default and cannot enable data in production', () => {
  assert.equal(resolveDevelopmentTestData('http://localhost:5173/#/timeline', true), true)
  assert.equal(resolveDevelopmentTestData('https://daygo.example/#/timeline?testData=on', false), false)
  assert.equal(resolveDevelopmentTestData('not a url', false), false)
})

test('keeps an explicit selection while the single-page app changes routes', () => {
  const select = createDevelopmentTestDataSelector(true)
  assert.equal(select('http://localhost:5173/#/timeline?testData=off'), false)
  assert.equal(select('http://localhost:5173/#/chat'), false)
  assert.equal(select('http://localhost:5173/#/settings?testData=on'), true)
  assert.equal(select('http://localhost:5173/#/daily'), true)
})
