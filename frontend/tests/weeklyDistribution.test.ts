import assert from 'node:assert/strict'
import test from 'node:test'

import type { WeeklyDashboardDTO } from '../src/api/dto'
import { buildWeeklyDistribution } from '../src/stores/weeklyDistribution'
import { currentWeekDayKey } from '../src/views/Timeline/weekLayout'

function dashboard(): WeeklyDashboardDTO {
  return {
    weekStart: '2026-10-05', weekStartTs: 0, weekEndTs: 604800,
    trackedMinutes: 0, focusMinutes: 0, categories: [], days: [],
    insights: { longestFocusMinutes: 0, longestFocusDay: '', peakHour: -1,
      peakHourMinutes: 0, mostActiveDay: '', mostActiveDayMinutes: 0,
      activeDays: 0, avgDailyFocusMinutes: 0 },
  }
}

test('week today follows the backend logical-day window after midnight', () => {
  const start = new Date(2026, 8, 21, 4).getTime() / 1000
  const end = new Date(2026, 8, 22, 4).getTime() / 1000
  const columns = [{ day: '2026-09-21', windowStartTs: start, windowEndTs: end }]
  assert.equal(currentWeekDayKey(columns, new Date(2026, 8, 22, 2).getTime() / 1000), '2026-09-21')
  assert.equal(currentWeekDayKey(columns, end), '')
})

// The current distribution card consumes only weekly category totals.
test('weekly distribution keeps valid totals and sanitizes category names and colors', () => {
  const input = dashboard()
  input.categories = [
    { name: ' Work ', minutes: 60, share: 1, colorHex: '#4f8cff' },
    { name: 'Personal', minutes: 30, share: 0, colorHex: 'url(unsafe)' },
    { name: ' ', minutes: 20, share: 0, colorHex: '' },
    { name: 'Negative', minutes: -1, share: 0, colorHex: '' },
    { name: 'Invalid', minutes: Number.NaN, share: 0, colorHex: '' },
    { name: 'Infinite', minutes: Number.POSITIVE_INFINITY, share: 0, colorHex: '' },
  ]
  const categories = buildWeeklyDistribution(input).categories
  assert.deepEqual(categories.map(({ name, minutes, colorHex }) => ({ name, minutes, colorHex })), [
    { name: 'Work', minutes: 60, colorHex: '#4f8cff' },
    { name: 'Personal', minutes: 30, colorHex: '#7D7A84' },
  ])
})

test('weekly distribution does not read unused day segments or insight fields', () => {
  const input = dashboard()
  Object.defineProperty(input, 'days', { get: () => { throw new Error('unused day traversal') } })
  Object.defineProperty(input, 'insights', { get: () => { throw new Error('unused insight traversal') } })
  assert.deepEqual(buildWeeklyDistribution(input).categories, [])
})
