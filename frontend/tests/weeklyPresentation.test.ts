import assert from 'node:assert/strict'
import test from 'node:test'

import type { WeeklyDashboardDTO } from '../src/api/dto'
import { buildWeeklyPresentation } from '../src/stores/weeklyPresentation'
import { currentWeekDayKey } from '../src/views/Timeline/weekLayout'

test('rhythm folds a segment crossing midnight into the next clock hour', () => {
  const startTs = new Date(2026, 8, 21, 23, 50).getTime() / 1000
  const dashboard: WeeklyDashboardDTO = {
    weekStart: '2026-09-21', weekStartTs: startTs, weekEndTs: startTs + 604800,
    trackedMinutes: 20, focusMinutes: 20, categories: [],
    days: [{ day: '2026-09-21', trackedMinutes: 20, focusMinutes: 20,
      categories: [], segments: [{ startTs, endTs: startTs + 1200, category: 'Work', isIdle: false }] }],
    insights: { longestFocusMinutes: 0, longestFocusDay: '', peakHour: 0,
      peakHourMinutes: 0, mostActiveDay: '', mostActiveDayMinutes: 0,
      activeDays: 0, avgDailyFocusMinutes: 0 },
  }
  const result = buildWeeklyPresentation(dashboard)
  assert.equal(result.rhythm[23]?.focusMinutes, 10)
  assert.equal(result.rhythm[0]?.focusMinutes, 10)
})

test('week today follows the backend logical-day window after midnight', () => {
  const start = new Date(2026, 8, 21, 4).getTime() / 1000
  const end = new Date(2026, 8, 22, 4).getTime() / 1000
  const columns = [{ day: '2026-09-21', windowStartTs: start, windowEndTs: end }]
  assert.equal(currentWeekDayKey(columns, new Date(2026, 8, 22, 2).getTime() / 1000), '2026-09-21')
  assert.equal(currentWeekDayKey(columns, end), '')
})
