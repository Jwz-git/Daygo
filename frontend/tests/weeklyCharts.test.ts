import assert from 'node:assert/strict'
import test from 'node:test'

import type { WeeklyDashboardDTO, WeeklySegmentDTO } from '../src/api/dto'
import {
  activityWindow,
  buildContextCharts,
  buildHeatmap,
  buildSankey,
  buildTreemap,
  buildWorkflow,
  logicalMinute,
  sankeyAppColor,
  weeklyChartFacts,
} from '../src/stores/weeklyCharts'

/*
 * Fixture week (local time, logical days start at 04:00):
 *   Mon 10:00–11:00 Coding  github.com, distraction 10:20–10:30
 *   Mon 11:05–11:35 Distraction  youtube.com
 *   Mon 11:35–12:00 Writing  (no app)
 *   Mon 00:30–01:00 next calendar day, still Monday's logical day: Coding, VS Code
 *   Tue 10:00–10:30 Coding  github.com
 * Previous week: Mon 10:00–10:20 Coding github.com.
 * Expected values below are worked out by hand from Dayflow's rules.
 */

function at(day: number, hour: number, minute: number): number {
  return Math.floor(new Date(2026, 8, day, hour, minute, 0).getTime() / 1000)
}

function segment(
  start: number,
  end: number,
  category: string,
  primary: string | null,
  distractions: Array<[number, number]> = [],
): WeeklySegmentDTO {
  return {
    startTs: start,
    endTs: end,
    category,
    isIdle: false,
    appSites: primary === null ? null : { primary, secondary: null },
    distractions: distractions.map(([startTs, endTs]) => ({ startTs, endTs })),
  }
}

function week(days: WeeklySegmentDTO[][]): WeeklyDashboardDTO {
  const padded = Array.from({ length: 7 }, (_, index) => days[index] ?? [])
  return {
    weekStart: '2026-09-14',
    weekStartTs: at(14, 4, 0),
    weekEndTs: at(21, 4, 0),
    trackedMinutes: 0,
    focusMinutes: 0,
    categories: [
      { name: 'Coding', minutes: 0, share: 0, colorHex: '#4F8CFF' },
      { name: 'Distraction', minutes: 0, share: 0, colorHex: '#FF6B6B' },
      { name: 'Writing', minutes: 0, share: 0, colorHex: '#59C48C' },
    ],
    days: padded.map((segments, index) => ({
      day: `2026-09-${14 + index}`,
      trackedMinutes: 0,
      focusMinutes: 0,
      categories: [],
      segments,
    })),
    insights: {
      longestFocusMinutes: 0, longestFocusDay: '', peakHour: -1, peakHourMinutes: 0,
      mostActiveDay: '', mostActiveDayMinutes: 0, activeDays: 0, avgDailyFocusMinutes: 0,
    },
  }
}

const current = week([
  [
    segment(at(14, 10, 0), at(14, 11, 0), 'Coding', 'github.com', [[at(14, 10, 20), at(14, 10, 30)]]),
    segment(at(14, 11, 5), at(14, 11, 35), 'Distraction', 'youtube.com'),
    segment(at(14, 11, 35), at(14, 12, 0), 'Writing', null),
    segment(at(15, 0, 30), at(15, 1, 0), 'Coding', 'Visual Studio Code'),
  ],
  [segment(at(15, 10, 0), at(15, 10, 30), 'Coding', 'github.com')],
])
const previous = week([[segment(at(7, 10, 0), at(7, 10, 20), 'Coding', 'github.com')]])
const facts = weeklyChartFacts(current)

test('facts sit on the logical-day axis with the timeline app identity', () => {
  assert.equal(logicalMinute(at(14, 10, 0)), 600)
  assert.equal(logicalMinute(at(15, 0, 30)), 1470, 'after midnight stays to the right of the evening')
  const monday = facts.filter((fact) => fact.dayIndex === 0)
  assert.deepEqual(monday.map((fact) => fact.startMinute), [600, 665, 695, 1470])
  assert.deepEqual(monday.map((fact) => fact.appKey), ['brand:github', 'brand:youtube', 'other', 'brand:vscode'])
  assert.deepEqual(monday.map((fact) => fact.appName), ['GitHub', 'YouTube', '', 'Visual Studio Code'])
  assert.deepEqual(monday[0].distractions, [{ startMinute: 620, endMinute: 630 }])
  assert.equal(monday[1].isDistractionCategory, true)
})

test('activity window pads 30 minutes and snaps to 15', () => {
  assert.deepEqual(activityWindow(facts), { start: 570, end: 1530 })
})

test('workflow takes the dominant bucket per 15-minute cell', () => {
  const workflow = buildWorkflow(facts)
  assert.equal(workflow.rows.length, 7)
  assert.equal(workflow.rows[0].length, 64) // (1530 - 570) / 15
  assert.deepEqual(workflow.rows[0][2], { category: 'Coding', colorHex: '#4F8CFF', minutes: 15, occupancy: 1 }) // 10:00
  const elevenOClock = workflow.rows[0][6] // 11:00–11:15: Coding ended, Distraction 11:05–11:15
  assert.equal(elevenOClock.category, 'Distraction')
  assert.equal(elevenOClock.colorHex, '#FF5950', 'distraction uses its own fixed colour')
  assert.equal(elevenOClock.occupancy, 10 / 15)
  assert.deepEqual(workflow.rows[0][0], { category: null, colorHex: null, minutes: 0, occupancy: 0 })
  assert.deepEqual(workflow.totals.map((total) => [total.name, total.minutes]), [
    ['Coding', 120], ['Distraction', 30], ['Writing', 25],
  ])
})

test('heatmap scores focus negative and distraction positive', () => {
  const heatmap = buildHeatmap(facts)
  assert.equal(heatmap.rows[0].length, 192) // (1530 - 570) / 5
  const bucket = (minute: number) => heatmap.rows[0][(minute - 570) / 5]
  assert.ok(bucket(600) < 0, 'focused coding scores below zero')
  assert.ok(bucket(620) > 0, 'the recorded distraction scores above zero')
  assert.ok(bucket(670) > 0, 'a Distraction-category block scores above zero')
  assert.equal(bucket(580), 0, 'an empty bucket is neutral')
  assert.ok(heatmap.rows[2].every((value) => value === 0), 'a day without data is all neutral')
})

test('context charts count category switches and distractions per day', () => {
  const context = buildContextCharts(facts)
  // Coding → Distraction → Writing → Coding: three switches on Monday.
  assert.deepEqual(context.days[0], { dayIndex: 0, shifts: 3, distracted: 2 })
  assert.deepEqual(context.days[1], { dayIndex: 1, shifts: 0, distracted: 0 })
  assert.deepEqual(context.busiest, { dayIndex: 0, shifts: 3, distracted: 2 })
  // Only 10:00–18:00 events are plotted; the 00:30 switch is outside.
  assert.deepEqual(
    context.events.map((event) => [event.kind, event.minute]),
    [['context', 665], ['context', 695], ['distraction', 620], ['distraction', 665]],
  )
  assert.equal(buildContextCharts([]).busiest, null)
})

test('treemap groups apps per category with the change against last week', () => {
  const treemap = buildTreemap(facts, weeklyChartFacts(previous))
  assert.deepEqual(treemap.map((category) => [category.name, category.minutes]), [
    ['Coding', 120], ['Distraction', 30], ['Writing', 25],
  ])
  assert.deepEqual(
    treemap[0].apps.map((app) => [app.key, app.minutes, app.changeMinutes]),
    [['brand:github', 90, 70], ['brand:vscode', 30, null]],
  )
})

test('sankey links categories to apps and conserves minutes', () => {
  const sankey = buildSankey(facts)
  assert.equal(sankey.total, 175)
  const linked = sankey.links.reduce((sum, link) => sum + link.minutes, 0)
  assert.equal(linked, 175, 'every visible minute flows through exactly one link')
  const github = sankey.links.find((link) => link.from === 'Coding' && link.to === 'brand:github')
  assert.equal(github?.minutes, 90)
  assert.deepEqual(sankey.apps.map((app) => app.key), ['brand:github', 'brand:vscode', 'brand:youtube', 'other'])
})

// Golden values from Dayflow's own WeeklyDashboardBuilder.appColorHex, run in
// Swift (brand needles first, then the djb2 hash over the name's UTF-8 bytes).
test('sankey app colours follow Dayflow per app, not per category', () => {
  assert.equal(sankeyAppColor('GitHub'), '#24292F')
  assert.equal(sankeyAppColor('YouTube'), '#FF0000')
  assert.equal(sankeyAppColor('Claude'), '#D97757')
  assert.equal(sankeyAppColor('Code'), '#6CDACD')
  assert.equal(sankeyAppColor('Visual Studio Code'), '#FFC6B7')
  assert.equal(sankeyAppColor('linear.app'), '#5E6AD2', 'brand needle wins before the hash')
  assert.equal(sankeyAppColor('developer.mozilla.org'), '#DE9DFC')
  assert.equal(sankeyAppColor('bilibili'), '#FFA189')
  assert.equal(sankeyAppColor('Wechat'), '#BFB6AE')
  assert.equal(sankeyAppColor('微信'), '#6CDACD')
  // Deliberate difference: Dayflow matches "x" anywhere, painting Firefox black.
  assert.equal(sankeyAppColor('X'), '#111111')
  assert.notEqual(sankeyAppColor('Firefox'), '#111111')

  const sankey = buildSankey(facts)
  const color = new Map(sankey.apps.map((app) => [app.key, app.colorHex]))
  assert.equal(color.get('brand:github'), '#24292F')
  assert.equal(color.get('brand:youtube'), '#FF0000')
  assert.equal(color.get('other'), '#D9D9D9')
})

test('an app used under two categories is one node fed by two links', () => {
  const crossed = weeklyChartFacts(week([[
    segment(at(14, 10, 0), at(14, 11, 0), 'Coding', 'github.com'),
    segment(at(14, 11, 0), at(14, 11, 30), 'Writing', 'github.com'),
  ]]))
  const sankey = buildSankey(crossed)
  assert.deepEqual(sankey.apps.map((app) => [app.key, app.minutes]), [['brand:github', 90]])
  assert.deepEqual(
    sankey.links.map((link) => [link.from, link.to, link.minutes]).sort(),
    [['Coding', 'brand:github', 60], ['Writing', 'brand:github', 30]],
  )
})
