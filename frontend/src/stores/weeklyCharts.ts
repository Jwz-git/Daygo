/*
 * Chart snapshots for the weekly page, ported from Dayflow's
 * WeeklyDashboardBuilder (Core/Weekly, MIT, Copyright (c) 2025 Jerry Liu).
 *
 * Every chart is computed from the weekly payload's per-day segments: time,
 * category, the card's app/site pair and its distraction intervals (resolved
 * to instants by Go). The only differences from Dayflow are deliberate:
 *
 *   - App identity uses the timeline's own resolver (lib/appSiteIcon), so a
 *     chart shows the same app name and icon as the card it came from.
 *   - "Distraction" is the Distraction category or the card's recorded
 *     distraction intervals. Dayflow also scans titles for the English word
 *     "distraction", which never matches non-English cards.
 *
 * Minutes are on the logical-day axis: 04:00 is 240 and 01:30 after midnight
 * is 1530, so a day's late night stays to the right of its evening.
 */

import type { WeeklyDashboardDTO, WeeklySegmentDTO } from '@/api/dto'
import { appSiteValues, resolveAppSiteIdentity, type AppSiteIconKind } from '@/lib/appSiteIcon'

const DAY_START_MINUTE = 4 * 60
const DAY_END_MINUTE = 28 * 60
const OTHER_KEY = 'other'
const OTHER_COLOR = '#BFB6AE'
const DISTRACTION_COLOR = '#FF5950'
const FALLBACK_COLOR = '#7D7A84'

// --------------------------------------------------------------------------
// Facts
// --------------------------------------------------------------------------

export interface WeeklyChartFact {
  /** 0..6, Monday first, by position in the weekly payload. */
  dayIndex: number
  startMinute: number
  endMinute: number
  minutes: number
  category: string
  colorHex: string
  isIdle: boolean
  isDistractionCategory: boolean
  appKey: string
  appName: string
  /** The values the timeline icon resolver takes, for drawing the app mark. */
  appSites: string[]
  distractions: Array<{ startMinute: number; endMinute: number }>
}

const BRAND_NAMES: Partial<Record<AppSiteIconKind, string>> = {
  chatgpt: 'ChatGPT',
  chrome: 'Chrome',
  claude: 'Claude',
  ghostty: 'Ghostty',
  gemini: 'Gemini',
  iterm2: 'iTerm2',
  cursor: 'Cursor',
  daygo: 'Daygo',
  discord: 'Discord',
  figma: 'Figma',
  finder: 'Finder',
  github: 'GitHub',
  'google-docs': 'Google Docs',
  messages: 'Messages',
  notes: 'Notes',
  notion: 'Notion',
  safari: 'Safari',
  slack: 'Slack',
  terminal: 'Terminal',
  vscode: 'Visual Studio Code',
  warp: 'Warp',
  xcode: 'Xcode',
  youtube: 'YouTube',
  bilibili: 'bilibili',
}

export function isDistractionCategoryName(name: string): boolean {
  const normalized = name.trim().toLowerCase()
  return normalized === 'distraction' || normalized === 'distractions'
}

function safeColor(value: string | undefined): string {
  return value && /^#[0-9a-f]{6}$/i.test(value) ? value : FALLBACK_COLOR
}

/** Minute on the logical-day axis (04:00 = 240, past midnight > 1440). */
export function logicalMinute(ts: number): number {
  const date = new Date(ts * 1000)
  const minute = date.getHours() * 60 + date.getMinutes() + date.getSeconds() / 60
  return minute < DAY_START_MINUTE ? minute + 1440 : minute
}

function appIdentity(segment: WeeklySegmentDTO): { key: string; name: string; sites: string[] } {
  const sites = appSiteValues(segment.appSites ?? null)
  const value = sites[0]
  if (value === undefined) return { key: OTHER_KEY, name: '', sites: [] }
  const identity = resolveAppSiteIdentity(value)
  if (identity.kind !== 'generic') {
    return { key: `brand:${identity.kind}`, name: BRAND_NAMES[identity.kind] ?? identity.label, sites }
  }
  const key = (identity.host ?? identity.label).toLocaleLowerCase('en-US')
  // The icon resolver turns a dotless name into "<name>.com"; show the name
  // as written then, and a real site without its public suffix.
  const name = identity.host !== null && identity.label.includes('.') ? siteName(identity.host) : identity.label
  return { key: `app:${key}`, name, sites }
}

// Second-level labels that belong to the suffix ("hdu.edu.cn" → "hdu").
const SECOND_LEVEL_SUFFIXES = new Set(['ac', 'co', 'com', 'edu', 'gov', 'net', 'org'])

/** "developer.mozilla.org" → "developer.mozilla", "course.hdu.edu.cn" → "course.hdu". */
function siteName(host: string): string {
  const labels = host.replace(/^www\./, '').split('.')
  if (labels.length < 2) return host
  labels.pop()
  if (labels.length >= 2 && SECOND_LEVEL_SUFFIXES.has(labels[labels.length - 1])) labels.pop()
  return labels.join('.')
}

/** Folds a weekly payload into one fact per non-System segment. */
export function weeklyChartFacts(dashboard: WeeklyDashboardDTO): WeeklyChartFact[] {
  const colors = new Map(dashboard.categories.map((category) => [category.name, category.colorHex]))
  const facts: WeeklyChartFact[] = []
  dashboard.days.slice(0, 7).forEach((day, dayIndex) => {
    for (const segment of day.segments) {
      if (segment.category === 'System') continue
      const minutes = Math.max(0, (segment.endTs - segment.startTs) / 60)
      if (!(minutes > 0)) continue
      const startMinute = logicalMinute(segment.startTs)
      const app = appIdentity(segment)
      facts.push({
        dayIndex,
        startMinute,
        endMinute: startMinute + minutes,
        minutes,
        category: segment.category,
        colorHex: safeColor(colors.get(segment.category)),
        isIdle: segment.isIdle,
        isDistractionCategory: isDistractionCategoryName(segment.category),
        appKey: app.key,
        appName: app.name,
        appSites: app.sites,
        distractions: (segment.distractions ?? [])
          .map((interval) => ({
            startMinute: startMinute + (interval.startTs - segment.startTs) / 60,
            endMinute: startMinute + (interval.endTs - segment.startTs) / 60,
          }))
          .filter((interval) => interval.endMinute > interval.startMinute),
      })
    }
  })
  return facts.sort((left, right) => left.dayIndex - right.dayIndex || left.startMinute - right.startMinute)
}

function visible(facts: WeeklyChartFact[]): WeeklyChartFact[] {
  return facts.filter((fact) => !fact.isIdle)
}

/** Dayflow's weeklyActivityWindow: padded, snapped to 15 minutes, clamped. */
export function activityWindow(facts: WeeklyChartFact[]): { start: number; end: number } {
  if (facts.length === 0) return { start: 9 * 60, end: 22 * 60 }
  const earliest = Math.min(...facts.map((fact) => fact.startMinute))
  const latest = Math.max(...facts.map((fact) => fact.endMinute))
  const start = Math.floor(Math.max(DAY_START_MINUTE, earliest - 30) / 15) * 15
  let end = Math.ceil(Math.min(DAY_END_MINUTE, latest + 30) / 15) * 15
  if (end === 24 * 60) end = DAY_END_MINUTE
  return end > start ? { start, end } : { start: 9 * 60, end: 22 * 60 }
}

/** Whole-hour tick positions inside a window, on the logical-day axis. */
export function hourTicks(start: number, end: number): number[] {
  const ticks: number[] = []
  for (let minute = Math.ceil(start / 60) * 60; minute <= end; minute += 60) ticks.push(minute)
  return ticks
}

function overlap(start: number, end: number, from: number, to: number): number {
  return Math.max(0, Math.min(end, to) - Math.max(start, from))
}

// --------------------------------------------------------------------------
// Weekly workflow: 15-minute cells, dominant category per cell
// --------------------------------------------------------------------------

export interface WeeklyWorkflowCell {
  category: string | null
  colorHex: string | null
  minutes: number
  occupancy: number
}

export interface WeeklyWorkflowSnapshot {
  start: number
  end: number
  slotMinutes: number
  rows: WeeklyWorkflowCell[][]
  totals: Array<{ name: string; colorHex: string; minutes: number }>
}

function workflowBucket(fact: WeeklyChartFact): { id: string; name: string; colorHex: string } {
  if (fact.isDistractionCategory) return { id: 'distraction', name: fact.category, colorHex: DISTRACTION_COLOR }
  return { id: fact.category, name: fact.category, colorHex: fact.colorHex }
}

export function buildWorkflow(facts: WeeklyChartFact[]): WeeklyWorkflowSnapshot {
  const shown = visible(facts)
  const window = activityWindow(shown)
  const slotMinutes = 15
  const slotCount = Math.max(1, Math.floor((window.end - window.start) / slotMinutes))
  const rows = Array.from({ length: 7 }, (_, dayIndex) => {
    const dayFacts = shown.filter((fact) => fact.dayIndex === dayIndex)
    return Array.from({ length: slotCount }, (_, slot): WeeklyWorkflowCell => {
      const from = window.start + slot * slotMinutes
      const to = from + slotMinutes
      const buckets = new Map<string, { name: string; colorHex: string; minutes: number }>()
      for (const fact of dayFacts) {
        const minutes = overlap(fact.startMinute, fact.endMinute, from, to)
        if (minutes <= 0) continue
        const bucket = workflowBucket(fact)
        const entry = buckets.get(bucket.id) ?? { name: bucket.name, colorHex: bucket.colorHex, minutes: 0 }
        entry.minutes += minutes
        buckets.set(bucket.id, entry)
      }
      const total = [...buckets.values()].reduce((sum, entry) => sum + entry.minutes, 0)
      const dominant = [...buckets.values()]
        .sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name))[0]
      if (dominant === undefined) return { category: null, colorHex: null, minutes: 0, occupancy: 0 }
      return {
        category: dominant.name,
        colorHex: dominant.colorHex,
        minutes: Math.round(total),
        occupancy: Math.min(1, total / slotMinutes),
      }
    })
  })
  const totals = new Map<string, { name: string; colorHex: string; minutes: number }>()
  for (const fact of shown) {
    const bucket = workflowBucket(fact)
    const entry = totals.get(bucket.id) ?? { name: bucket.name, colorHex: bucket.colorHex, minutes: 0 }
    entry.minutes += fact.minutes
    totals.set(bucket.id, entry)
  }
  return {
    start: window.start,
    end: window.start + slotCount * slotMinutes,
    slotMinutes,
    rows,
    totals: [...totals.values()]
      .filter((entry) => entry.minutes > 0)
      .sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name))
      .slice(0, 7),
  }
}

// --------------------------------------------------------------------------
// Focus heatmap: 5-minute buckets scored -1 (focused) .. +1 (distracted)
// --------------------------------------------------------------------------

export interface WeeklyHeatmapSnapshot {
  start: number
  end: number
  bucketMinutes: number
  rows: number[][]
}

function addInterval(values: number[], start: number, end: number, origin: number, bucket: number, weight = 1): void {
  const windowEnd = origin + values.length * bucket
  const from = Math.max(start, origin)
  const to = Math.min(end, windowEnd)
  if (to <= from) return
  const first = Math.max(0, Math.floor((from - origin) / bucket))
  const last = Math.min(values.length - 1, Math.ceil((to - origin) / bucket) - 1)
  for (let index = first; index <= last; index += 1) {
    const bucketStart = origin + index * bucket
    values[index] += overlap(from, to, bucketStart, bucketStart + bucket) * weight
  }
}

function runLengths(values: boolean[]): number[] {
  const lengths = values.map(() => 0)
  let index = 0
  while (index < values.length) {
    if (!values[index]) {
      index += 1
      continue
    }
    const start = index
    while (index < values.length && values[index]) index += 1
    for (let run = start; run < index; run += 1) lengths[run] = index - start
  }
  return lengths
}

function smoothFocused(scores: number[]): number[] {
  if (scores.length <= 2) return scores
  return scores.map((value, index) => {
    if (value >= 0) return value
    const neighbours = scores.slice(Math.max(0, index - 1), Math.min(scores.length, index + 2)).filter((score) => score < 0)
    if (neighbours.length <= 1) return value
    const average = neighbours.reduce((sum, score) => sum + score, 0) / neighbours.length
    return Math.max(-1, Math.min(1, value * 0.55 + average * 0.45))
  })
}

function heatmapRow(dayFacts: WeeklyChartFact[], origin: number, bucket: number, count: number): number[] {
  const focus = Array<number>(count).fill(0)
  const distraction = Array<number>(count).fill(0)
  const switches = Array<number>(count).fill(0)
  let previous: WeeklyChartFact | null = null
  for (const fact of dayFacts) {
    if (
      previous !== null &&
      fact.startMinute - previous.endMinute <= 20 &&
      `${fact.category}|${fact.appKey}` !== `${previous.category}|${previous.appKey}`
    ) {
      const index = Math.floor((fact.startMinute - origin) / bucket)
      if (index >= 0 && index < count) switches[index] += 1
    }
    if (fact.isDistractionCategory) {
      addInterval(distraction, fact.startMinute, fact.endMinute, origin, bucket)
    } else {
      addInterval(focus, fact.startMinute, fact.endMinute, origin, bucket)
      for (const interval of fact.distractions) {
        addInterval(distraction, interval.startMinute, interval.endMinute, origin, bucket)
        addInterval(focus, interval.startMinute, interval.endMinute, origin, bucket, -1)
      }
    }
    previous = fact
  }
  const clean = focus.map((minutes, index) => minutes >= bucket * 0.6 && distraction[index] < 1)
  const runs = runLengths(clean)
  const raw = focus.map((minutes, index) => {
    const focusRatio = Math.max(0, Math.min(1, minutes / bucket))
    const distractionRatio = Math.max(0, Math.min(1, distraction[index] / bucket))
    const sustained = Math.min(1, runs[index] / 6)
    const focusStrength = focusRatio * (0.35 + 0.65 * sustained)
    const distractionStrength = Math.min(1, distractionRatio * 1.25)
    const switchStrength = Math.min(1, switches[index] / 2) * 0.22
    return Math.max(-1, Math.min(1, distractionStrength + switchStrength - focusStrength))
  })
  return smoothFocused(raw)
}

export function buildHeatmap(facts: WeeklyChartFact[]): WeeklyHeatmapSnapshot {
  const shown = visible(facts)
  const window = activityWindow(shown)
  const bucketMinutes = 5
  const count = Math.max(1, Math.ceil((window.end - window.start) / bucketMinutes))
  return {
    start: window.start,
    end: window.start + count * bucketMinutes,
    bucketMinutes,
    rows: Array.from({ length: 7 }, (_, dayIndex) =>
      heatmapRow(shown.filter((fact) => fact.dayIndex === dayIndex), window.start, bucketMinutes, count)),
  }
}

// --------------------------------------------------------------------------
// Context shifts and distractions
// --------------------------------------------------------------------------

export interface WeeklyContextEvent {
  dayIndex: number
  kind: 'context' | 'distraction'
  minute: number
}

export interface WeeklyContextSnapshot {
  /** Distribution window, 10:00–18:00 as in Dayflow. */
  start: number
  end: number
  events: WeeklyContextEvent[]
  days: Array<{ dayIndex: number; shifts: number; distracted: number }>
  /** The day with the most shifts + distractions, or null when there are none. */
  busiest: { dayIndex: number; shifts: number; distracted: number } | null
}

export function buildContextCharts(facts: WeeklyChartFact[]): WeeklyContextSnapshot {
  const start = 10 * 60
  const end = 18 * 60
  const inWindow = (minute: number): boolean => minute >= start && minute <= end
  const events: WeeklyContextEvent[] = []
  const days = Array.from({ length: 7 }, (_, dayIndex) => {
    const dayFacts = visible(facts).filter((fact) => fact.dayIndex === dayIndex)
    let shifts = 0
    let previous: string | null = null
    for (const fact of dayFacts) {
      if (previous !== null && previous !== fact.category) {
        shifts += 1
        if (inWindow(fact.startMinute)) events.push({ dayIndex, kind: 'context', minute: fact.startMinute })
      }
      previous = fact.category
    }
    const distractionTimes = dayFacts.flatMap((fact) => {
      if (fact.distractions.length > 0) return fact.distractions.map((interval) => interval.startMinute)
      return fact.isDistractionCategory ? [fact.startMinute] : []
    })
    for (const minute of distractionTimes) {
      if (inWindow(minute)) events.push({ dayIndex, kind: 'distraction', minute })
    }
    return { dayIndex, shifts, distracted: distractionTimes.length }
  })
  const busiest = days.reduce<WeeklyContextSnapshot['busiest']>((best, day) => {
    const score = day.shifts + day.distracted
    if (score === 0) return best
    if (best === null || score > best.shifts + best.distracted) return day
    return best
  }, null)
  return { start, end, events: events.slice(0, 80), days, busiest }
}

// --------------------------------------------------------------------------
// Most used per category (treemap) and category → app flow (sankey)
// --------------------------------------------------------------------------

export interface WeeklyTreemapApp {
  key: string
  name: string
  sites: string[]
  minutes: number
  /** Minutes against last week; null when the app had no time last week. */
  changeMinutes: number | null
}

export interface WeeklyTreemapCategory {
  name: string
  colorHex: string
  minutes: number
  apps: WeeklyTreemapApp[]
}

function appMinutesByCategory(facts: WeeklyChartFact[]): Map<string, number> {
  const result = new Map<string, number>()
  for (const fact of visible(facts)) {
    const key = `${fact.category}|${fact.appKey}`
    result.set(key, (result.get(key) ?? 0) + fact.minutes)
  }
  return result
}

export function buildTreemap(facts: WeeklyChartFact[], previousFacts: WeeklyChartFact[]): WeeklyTreemapCategory[] {
  const previous = appMinutesByCategory(previousFacts)
  const byCategory = new Map<string, WeeklyChartFact[]>()
  for (const fact of visible(facts)) {
    const list = byCategory.get(fact.category) ?? []
    list.push(fact)
    byCategory.set(fact.category, list)
  }
  const categories = [...byCategory.entries()].map(([name, categoryFacts]): WeeklyTreemapCategory => {
    const byApp = new Map<string, WeeklyTreemapApp>()
    for (const fact of categoryFacts) {
      const app = byApp.get(fact.appKey) ?? {
        key: fact.appKey, name: fact.appName, sites: fact.appSites, minutes: 0, changeMinutes: null,
      }
      app.minutes += fact.minutes
      if (app.sites.length === 0 && fact.appSites.length > 0) app.sites = fact.appSites
      byApp.set(fact.appKey, app)
    }
    const apps = [...byApp.values()]
      .map((app) => {
        const last = previous.get(`${name}|${app.key}`) ?? 0
        return { ...app, changeMinutes: last > 0 ? Math.round(app.minutes - last) : null }
      })
      .sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name))
      .slice(0, 8)
    return {
      name,
      colorHex: categoryFacts[0]?.colorHex ?? FALLBACK_COLOR,
      minutes: categoryFacts.reduce((sum, fact) => sum + fact.minutes, 0),
      apps,
    }
  })
  return categories
    .filter((category) => category.apps.length > 0)
    .sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name))
    .slice(0, 5)
}

export interface WeeklySankeyNode {
  key: string
  name: string
  colorHex: string
  minutes: number
  sites: string[]
}

export interface WeeklySankeySnapshot {
  total: number
  categories: WeeklySankeyNode[]
  apps: WeeklySankeyNode[]
  links: Array<{ from: string; to: string; minutes: number }>
}

// Dayflow's WeeklyDashboardBuilder.appColorHex: a brand colour when the app
// name contains a known needle, otherwise a palette slot from the djb2 hash of
// the name (Swift Int arithmetic, so 64-bit wrapping). One deliberate change:
// "x" must be the whole name or a domain label, so "Firefox" is not painted
// as X.
const APP_COLOR_NEEDLES: Array<[RegExp, string]> = [
  [/chatgpt/, '#333333'],
  [/claude/, '#D97757'],
  [/codex/, '#111111'],
  [/cursor/, '#111111'],
  [/xcode/, '#4085FD'],
  [/dayflow|daygo/, '#FF7A2F'],
  [/figma/, '#FF7262'],
  [/slack/, '#36C5F0'],
  [/zoom/, '#4085FD'],
  [/meet/, '#34A853'],
  [/youtube/, '#FF0000'],
  [/reddit/, '#FF613C'],
  [/(^|[^a-z0-9])x([^a-z0-9]|$)|twitter/, '#111111'],
  [/substack/, '#FF6E3E'],
  [/notion/, '#111111'],
  [/linear/, '#5E6AD2'],
  [/github/, '#24292F'],
  [/safari/, '#2E8BFF'],
  [/chrome/, '#4285F4'],
  [/calendar/, '#A29993'],
  [/mail/, '#4F8EF7'],
  [/messages/, '#38D06E'],
  [/other/, '#D9D9D9'],
]
const APP_COLOR_PALETTE = ['#93BCFF', '#DE9DFC', '#6CDACD', '#FFA189', '#FFC6B7', OTHER_COLOR]
const APP_OTHER_COLOR = '#D9D9D9'

export function sankeyAppColor(name: string): string {
  const lowered = name.toLowerCase()
  const match = APP_COLOR_NEEDLES.find(([needle]) => needle.test(lowered))
  if (match !== undefined) return match[1]
  let hash = 5381n
  for (const byte of new TextEncoder().encode(name)) {
    hash = BigInt.asIntN(64, (hash << 5n) + hash + BigInt(byte))
  }
  const index = Number((hash < 0n ? -hash : hash) % BigInt(APP_COLOR_PALETTE.length))
  return APP_COLOR_PALETTE[index]
}

function topBuckets(
  nodes: WeeklySankeyNode[],
  maxVisible: number,
  otherColor: string,
): { nodes: WeeklySankeyNode[]; visible: Set<string> } {
  const sorted = [...nodes].sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name))
  if (sorted.length <= maxVisible) return { nodes: sorted, visible: new Set(sorted.map((node) => node.key)) }
  const kept = sorted.slice(0, maxVisible - 1).filter((node) => node.key !== OTHER_KEY)
  const otherMinutes = sorted.filter((node) => !kept.includes(node)).reduce((sum, node) => sum + node.minutes, 0)
  return {
    nodes: [...kept, { key: OTHER_KEY, name: '', colorHex: otherColor, minutes: otherMinutes, sites: [] }],
    visible: new Set(kept.map((node) => node.key)),
  }
}

export function buildSankey(facts: WeeklyChartFact[]): WeeklySankeySnapshot {
  const shown = visible(facts)
  const categoryNodes = new Map<string, WeeklySankeyNode>()
  const appNodes = new Map<string, WeeklySankeyNode>()
  for (const fact of shown) {
    const category = categoryNodes.get(fact.category) ?? {
      key: fact.category, name: fact.category, colorHex: fact.colorHex, minutes: 0, sites: [],
    }
    category.minutes += fact.minutes
    categoryNodes.set(fact.category, category)
    const app = appNodes.get(fact.appKey) ?? {
      key: fact.appKey,
      name: fact.appName,
      colorHex: fact.appKey === OTHER_KEY ? APP_OTHER_COLOR : sankeyAppColor(fact.appName),
      minutes: 0,
      sites: fact.appSites,
    }
    app.minutes += fact.minutes
    appNodes.set(fact.appKey, app)
  }
  const categories = topBuckets([...categoryNodes.values()], 6, OTHER_COLOR)
  const apps = topBuckets([...appNodes.values()], 10, APP_OTHER_COLOR)
  const links = new Map<string, number>()
  for (const fact of shown) {
    const from = categories.visible.has(fact.category) ? fact.category : OTHER_KEY
    const to = apps.visible.has(fact.appKey) ? fact.appKey : OTHER_KEY
    links.set(`${from}\u0000${to}`, (links.get(`${from}\u0000${to}`) ?? 0) + fact.minutes)
  }
  return {
    total: shown.reduce((sum, fact) => sum + fact.minutes, 0),
    categories: categories.nodes,
    apps: apps.nodes,
    links: [...links.entries()]
      .map(([key, minutes]) => {
        const [from, to] = key.split('\u0000')
        return { from, to, minutes }
      })
      .filter((link) => link.minutes > 0),
  }
}

export const WEEKLY_OTHER_KEY = OTHER_KEY
