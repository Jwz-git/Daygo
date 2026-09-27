import assert from 'node:assert/strict'
import test from 'node:test'

import type { WeeklyDashboardDTO } from '../src/api/dto'
import { buildWeeklyPresentation } from '../src/stores/weeklyPresentation'
import { currentWeekDayKey } from '../src/views/Timeline/weekLayout'

/*
 * The weekly charts run on a clock axis (minute-of-day) while the data is
 * grouped by the 04:00-based logical day, so a segment can cross midnight
 * inside one logical day. The rhythm fold must terminate for such a segment
 * (the 2026-09-27 freeze: the old cursor stalled at 1440) and wrap its tail
 * into hour 0 rather than dropping it.
 */

function localTimestamp(hour: number, minute: number): number {
  return Math.floor(new Date(2026, 8, 14, hour, minute, 0).getTime() / 1000)
}

function category(name: string, minutes: number) {
  return { name, minutes, share: 1, colorHex: '#4F8CFF' }
}

function dashboard(
  segments: WeeklyDashboardDTO['days'][number]['segments'],
  day = '2026-09-14',
): WeeklyDashboardDTO {
  const minutes = segments.reduce(
    (total, segment) => total + Math.max(0, (segment.endTs - segment.startTs) / 60),
    0,
  )
  const start = localTimestamp(0, 0)
  return {
    weekStart: day,
    weekStartTs: start,
    weekEndTs: start + 604800,
    trackedMinutes: minutes,
    focusMinutes: minutes,
    categories: [category('Coding', minutes)],
    days: [{
      day,
      trackedMinutes: minutes,
      focusMinutes: minutes,
      categories: [category('Coding', minutes)],
      segments,
    }],
    insights: {
      longestFocusMinutes: minutes,
      longestFocusDay: day,
      peakHour: 23,
      peakHourMinutes: minutes,
      mostActiveDay: day,
      mostActiveDayMinutes: minutes,
      activeDays: 1,
      avgDailyFocusMinutes: minutes,
    },
  }
}

function foldedMinutes(presentation: ReturnType<typeof buildWeeklyPresentation>): number {
  return presentation.rhythm.reduce(
    (total, slot) => total + slot.focusMinutes + slot.idleMinutes,
    0,
  )
}

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

test('a longer midnight crossing terminates and accounts for every minute', () => {
  const start = localTimestamp(23, 30)
  const presentation = buildWeeklyPresentation(
    dashboard([{ startTs: start, endTs: start + 45 * 60, category: 'Coding', isIdle: false }]),
  )

  // 23:30–24:00 stays in hour 23; 00:00–00:15 wraps into hour 0.
  assert.equal(presentation.rhythm[23]!.focusMinutes, 30)
  assert.equal(presentation.rhythm[0]!.focusMinutes, 15)
  assert.equal(foldedMinutes(presentation), 45)
})

test('an ordinary segment still splits across its clock hours', () => {
  const start = localTimestamp(9, 40)
  const presentation = buildWeeklyPresentation(
    dashboard([{ startTs: start, endTs: start + 50 * 60, category: 'Coding', isIdle: false }]),
  )

  assert.equal(presentation.rhythm[9]!.focusMinutes, 20)
  assert.equal(presentation.rhythm[10]!.focusMinutes, 30)
  assert.equal(foldedMinutes(presentation), 50)
})

test('a corrupted span cannot stall the fold', () => {
  const start = localTimestamp(11, 0)
  // A 400-day segment is not real data; the fold must stay bounded and
  // terminate instead of walking it hour by hour.
  const presentation = buildWeeklyPresentation(
    dashboard([{ startTs: start, endTs: start + 400 * 24 * 3600, category: 'Coding', isIdle: false }]),
  )

  assert.ok(foldedMinutes(presentation) <= 24 * 60)
})

test('an inverted or zero-length span is dropped instead of folded', () => {
  const start = localTimestamp(11, 0)
  const presentation = buildWeeklyPresentation(dashboard([
    { startTs: start, endTs: start, category: 'Coding', isIdle: false },
    { startTs: start, endTs: start - 600, category: 'Coding', isIdle: false },
  ]))

  assert.equal(presentation.days[0]!.segments.length, 0)
  assert.equal(foldedMinutes(presentation), 0)
})

test('idle and distraction minutes fold into the idle series', () => {
  const start = localTimestamp(14, 0)
  const presentation = buildWeeklyPresentation(dashboard([
    { startTs: start, endTs: start + 15 * 60, category: 'Idle', isIdle: true },
    { startTs: start + 15 * 60, endTs: start + 30 * 60, category: 'Distraction', isIdle: false },
  ]))

  assert.equal(presentation.rhythm[14]!.idleMinutes, 30)
  assert.equal(presentation.rhythm[14]!.focusMinutes, 0)
})

test('week today follows the backend logical-day window after midnight', () => {
  const start = new Date(2026, 8, 21, 4).getTime() / 1000
  const end = new Date(2026, 8, 22, 4).getTime() / 1000
  const columns = [{ day: '2026-09-21', windowStartTs: start, windowEndTs: end }]
  assert.equal(currentWeekDayKey(columns, new Date(2026, 8, 22, 2).getTime() / 1000), '2026-09-21')
  assert.equal(currentWeekDayKey(columns, end), '')
})
