import type { WeeklyDashboardDTO } from '@/api/dto'

export { shiftCalendarDate, shiftWeekStart } from '@/lib/calendarDate'

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

export function percentageLabel(value: number): string {
  return `${Math.round(clampedShare(value) * 100)}%`
}
