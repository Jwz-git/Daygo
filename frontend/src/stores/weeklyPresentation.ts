import type { WeeklyDashboardDTO } from '@/api/dto'

export { shiftCalendarDate, shiftWeekStart } from '@/lib/calendarDate'

export const WEEKLY_SERIES_COUNT = 6

export interface WeeklyCategoryPresentation {
  name: string
  minutes: number
  share: number
  colorHex: string
  seriesIndex: number
}

export interface WeeklyDaySegment {
  category: string
  colorHex: string
  isIdle: boolean
  /** Local minute-of-day, clipped to the visible window. */
  startMinute: number
  endMinute: number
  minutes: number
}

export interface WeeklyDayPresentation {
  /** yyyy-MM-dd logical day from the backend. */
  day: string
  /** 0..6, Monday=0. */
  weekday: number
  trackedMinutes: number
  focusMinutes: number
  categories: WeeklyCategoryPresentation[]
  segments: WeeklyDaySegment[]
}

export interface WeeklyRhythmSlot {
  hour: number
  focusMinutes: number
  idleMinutes: number
}

export interface WeeklyPresentation {
  trackedMinutes: number
  focusMinutes: number
  otherMinutes: number
  focusShare: number
  categories: WeeklyCategoryPresentation[]
  days: WeeklyDayPresentation[]
  /** Shared visible window [start, end) in local minute-of-day. */
  windowStartMinute: number
  windowEndMinute: number
  /** Focus/idle minutes folded into 24 local clock hours. */
  rhythm: WeeklyRhythmSlot[]
  insights: {
    longestFocusMinutes: number
    longestFocusDay: string
    peakHour: number
    peakHourMinutes: number
    mostActiveDay: string
    mostActiveDayMinutes: number
    activeDays: number
    avgDailyFocusMinutes: number
  }
}

function finiteNonNegative(value: number): number {
  return Number.isFinite(value) ? Math.max(0, value) : 0
}

function clampedShare(value: number): number {
  return Math.min(1, finiteNonNegative(value))
}

/** Monday=0 .. Sunday=6 for a yyyy-MM-dd string (pure calendar math, no zone). */
function weekdayIndex(day: string): number {
  const date = new Date(`${day}T12:00:00Z`)
  if (Number.isNaN(date.getTime())) return 0
  // 1970-01-01 was a Thursday (weekday 3 with Monday=0).
  const days = Math.floor(date.getTime() / 86400000)
  return (((days + 3) % 7) + 7) % 7
}

const WEEKDAY_SERIES = [0, 1, 2, 3, 4, 5, 6]

function presentationCategory(
  name: string,
  minutes: number,
  share: number,
  colorHex: string,
  seriesBase: number,
  seriesSeen: Map<string, number>,
): WeeklyCategoryPresentation {
  let seriesIndex = seriesSeen.get(name)
  if (seriesIndex === undefined) {
    seriesIndex = (seriesBase + seriesSeen.size) % WEEKLY_SERIES_COUNT
    seriesSeen.set(name, seriesIndex)
  }
  return {
    name: name.trim(),
    minutes: finiteNonNegative(minutes),
    share: clampedShare(share),
    colorHex: colorHex,
    seriesIndex,
  }
}

/**
 * Builds the chart-ready weekly presentation. All weekday/minute math runs on
 * timestamps from the backend; the only local calendar derivation is the
 * weekday label index, which is offset-independent (pure day-count math).
 */
export function buildWeeklyPresentation(dashboard: WeeklyDashboardDTO): WeeklyPresentation {
  const trackedMinutes = finiteNonNegative(dashboard.trackedMinutes)
  const focusMinutes = Math.min(trackedMinutes, finiteNonNegative(dashboard.focusMinutes))
  const categoryColors = new Map<string, string>()
  for (const category of dashboard.categories) {
    if (category.colorHex) categoryColors.set(category.name, category.colorHex)
  }

  // Day rows keyed by logical day; the backend always sends Mon..Sun but the
  // frontend tolerates gaps by keeping an empty row per weekday.
  const daysByWeekday = new Map<number, WeeklyDayPresentation>()
  const seriesSeen = new Map<string, number>()
  for (const day of dashboard.days) {
    const weekday = weekdayIndex(day.day)
    const dayTracked = finiteNonNegative(day.trackedMinutes)
    const segments: WeeklyDaySegment[] = day.segments
      .map((segment) => {
        const minutes = finiteNonNegative((segment.endTs - segment.startTs) / 60)
        return {
          category: segment.category,
          colorHex: segment.category === 'Idle'
            ? (categoryColors.get(segment.category) ?? '#C7C7CC')
            : categoryColors.get(segment.category) ?? '',
          isIdle: segment.isIdle,
          startMinute: localMinuteOfDay(segment.startTs),
          endMinute: localMinuteOfDay(segment.startTs) + minutes,
          minutes,
        }
      })
      .filter((segment) => segment.minutes > 0 && segment.category !== 'System')
      .sort((left, right) => left.startMinute - right.startMinute)

    daysByWeekday.set(weekday, {
      day: day.day,
      weekday,
      trackedMinutes: dayTracked,
      focusMinutes: Math.min(dayTracked, finiteNonNegative(day.focusMinutes)),
      categories: day.categories
        .map((category) => presentationCategory(
          category.name, category.minutes, category.share,
          category.colorHex || categoryColors.get(category.name) || '', 0, seriesSeen,
        ))
        .filter((category) => category.name.length > 0 && category.minutes > 0)
        .sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name)),
      segments,
    })
  }

  const days = WEEKDAY_SERIES.map((weekday) => daysByWeekday.get(weekday) ?? {
    day: '',
    weekday,
    trackedMinutes: 0,
    focusMinutes: 0,
    categories: [],
    segments: [],
  })

  // Shared visible window: earliest activity start to latest activity end,
  // padded to whole hours, clamped to a sane day span.
  let windowStart = Number.POSITIVE_INFINITY
  let windowEnd = Number.NEGATIVE_INFINITY
  for (const day of days) {
    for (const segment of day.segments) {
      windowStart = Math.min(windowStart, segment.startMinute)
      windowEnd = Math.max(windowEnd, segment.endMinute)
    }
  }
  if (!Number.isFinite(windowStart) || !Number.isFinite(windowEnd)) {
    windowStart = 9 * 60
    windowEnd = 18 * 60
  } else {
    windowStart = Math.max(0, Math.floor(windowStart / 60) * 60)
    windowEnd = Math.min(24 * 60, Math.ceil(windowEnd / 60) * 60)
    if (windowEnd - windowStart < 60) windowEnd = Math.min(24 * 60, windowStart + 60)
  }

  // Rhythm: focus vs idle minutes per local clock hour across the week.
  const rhythm: WeeklyRhythmSlot[] = Array.from({ length: 24 }, (_, hour) => ({
    hour,
    focusMinutes: 0,
    idleMinutes: 0,
  }))
  for (const day of days) {
    for (const segment of day.segments) {
      let at = segment.startMinute
      while (at < segment.endMinute) {
        const hour = Math.min(23, Math.floor(at / 60))
        const hourEnd = (hour + 1) * 60
        const overlap = Math.min(segment.endMinute, hourEnd) - at
        if (overlap > 0) {
          if (segment.isIdle) rhythm[hour].idleMinutes += overlap
          else rhythm[hour].focusMinutes += overlap
        }
        at = hourEnd
      }
    }
  }

  const categories = dashboard.categories
    .map((category) => presentationCategory(
      category.name, category.minutes, category.share, category.colorHex, 0, seriesSeen,
    ))
    .filter((category) => category.name.length > 0 && category.minutes > 0)
    .sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name))

  return {
    trackedMinutes,
    focusMinutes,
    otherMinutes: Math.max(0, trackedMinutes - focusMinutes),
    focusShare: trackedMinutes > 0 ? focusMinutes / trackedMinutes : 0,
    categories,
    days,
    windowStartMinute: windowStart,
    windowEndMinute: windowEnd,
    rhythm,
    insights: {
      longestFocusMinutes: finiteNonNegative(dashboard.insights?.longestFocusMinutes ?? 0),
      longestFocusDay: dashboard.insights?.longestFocusDay ?? '',
      peakHour: dashboard.insights?.peakHour ?? -1,
      peakHourMinutes: finiteNonNegative(dashboard.insights?.peakHourMinutes ?? 0),
      mostActiveDay: dashboard.insights?.mostActiveDay ?? '',
      mostActiveDayMinutes: finiteNonNegative(dashboard.insights?.mostActiveDayMinutes ?? 0),
      activeDays: Math.max(0, Math.round(dashboard.insights?.activeDays ?? 0)),
      avgDailyFocusMinutes: finiteNonNegative(dashboard.insights?.avgDailyFocusMinutes ?? 0),
    },
  }
}

export function percentageLabel(value: number): string {
  return `${Math.round(clampedShare(value) * 100)}%`
}

/** Local minute-of-day for a Unix second timestamp. */
export function localMinuteOfDay(ts: number): number {
  const date = new Date(ts * 1000)
  return date.getHours() * 60 + date.getMinutes()
}

/** Local hour label ("9:00") for an hour 0..23. */
export function hourLabel(hour: number): string {
  return `${hour}:00`
}
