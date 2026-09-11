import type { WeeklyDashboardDTO } from '@/api/dto'

export const WEEKLY_SERIES_COUNT = 6

export interface WeeklyCategoryPresentation {
  name: string
  minutes: number
  share: number
  seriesIndex: number
}

export interface WeeklyPresentation {
  trackedMinutes: number
  focusMinutes: number
  otherMinutes: number
  focusShare: number
  categories: WeeklyCategoryPresentation[]
}

function finiteNonNegative(value: number): number {
  return Number.isFinite(value) ? Math.max(0, value) : 0
}

function clampedShare(value: number): number {
  return Math.min(1, finiteNonNegative(value))
}

export function buildWeeklyPresentation(dashboard: WeeklyDashboardDTO): WeeklyPresentation {
  const trackedMinutes = finiteNonNegative(dashboard.trackedMinutes)
  const focusMinutes = Math.min(trackedMinutes, finiteNonNegative(dashboard.focusMinutes))
  const categories = dashboard.categories
    .map((category, index): WeeklyCategoryPresentation => {
      const minutes = finiteNonNegative(category.minutes)
      const derivedShare = trackedMinutes > 0 ? minutes / trackedMinutes : 0
      const sourceShare = Number.isFinite(category.share) ? category.share : derivedShare

      return {
        name: category.name.trim(),
        minutes,
        share: clampedShare(sourceShare),
        seriesIndex: index % WEEKLY_SERIES_COUNT,
      }
    })
    .filter((category) => category.name.length > 0 && category.minutes > 0)
    .sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name))

  return {
    trackedMinutes,
    focusMinutes,
    otherMinutes: Math.max(0, trackedMinutes - focusMinutes),
    focusShare: trackedMinutes > 0 ? focusMinutes / trackedMinutes : 0,
    categories,
  }
}

export function shiftCalendarDate(value: string, dayOffset: number): string | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (match === null) return null

  const year = Number(match[1])
  const month = Number(match[2]) - 1
  const day = Number(match[3])
  const date = new Date(Date.UTC(year, month, day))
  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month ||
    date.getUTCDate() !== day
  ) {
    return null
  }

  date.setUTCDate(date.getUTCDate() + dayOffset)
  const resultYear = date.getUTCFullYear()
  const resultMonth = String(date.getUTCMonth() + 1).padStart(2, '0')
  const resultDay = String(date.getUTCDate()).padStart(2, '0')
  return `${resultYear}-${resultMonth}-${resultDay}`
}

export function shiftWeekStart(value: string, weekOffset: number): string | null {
  return shiftCalendarDate(value, weekOffset * 7)
}

export function percentageLabel(value: number): string {
  return `${Math.round(clampedShare(value) * 100)}%`
}
