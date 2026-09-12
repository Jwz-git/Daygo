import assert from 'node:assert/strict'
import test from 'node:test'

import { safeTimeZone } from '../src/lib/timeZone'

test('keeps a valid IANA identifier untouched', () => {
  assert.equal(safeTimeZone('Asia/Shanghai'), 'Asia/Shanghai')
  assert.equal(safeTimeZone('America/New_York'), 'America/New_York')
  assert.equal(safeTimeZone('UTC'), 'UTC')
})

test('degrades a value Intl would reject to undefined', () => {
  // "Local" is exactly what Go's time.Local.String() emits; it must never reach
  // Intl.DateTimeFormat, which throws a RangeError that blanks the page.
  assert.equal(safeTimeZone('Local'), undefined)
  assert.equal(safeTimeZone('Not/AZone'), undefined)
  assert.equal(safeTimeZone('nonsense value'), undefined)
})

test('treats empty and nullish input as no preference', () => {
  assert.equal(safeTimeZone(''), undefined)
  assert.equal(safeTimeZone(null), undefined)
  assert.equal(safeTimeZone(undefined), undefined)
})
