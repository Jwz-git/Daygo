import assert from 'node:assert/strict'
import test from 'node:test'

import { delayUntilDayContextRefresh } from '../src/lib/dayContextRefresh'

test('schedules refresh at the backend-supplied logical-day end', () => {
  assert.equal(delayUntilDayContextRefresh(1_000, 999_250), 750)
})

test('refreshes immediately after a stale logical-day end', () => {
  assert.equal(delayUntilDayContextRefresh(1_000, 1_000_001), 0)
})
