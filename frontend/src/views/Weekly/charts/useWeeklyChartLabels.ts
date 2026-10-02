import { useI18n } from 'vue-i18n'

import { categoryLabel } from '@/lib/categoryLabel'
import { WEEKLY_OTHER_KEY } from '@/stores/weeklyCharts'

/*
 * Label formatting shared by the weekly charts. Weekday names come from the
 * backend's logical-day strings (never derived from "now"); hour labels are
 * localized clock hours for a minute on the logical-day axis.
 */
export function useWeeklyChartLabels(days: () => string[]) {
  const { t, locale } = useI18n()

  function weekday(dayIndex: number, style: 'short' | 'long' = 'short'): string {
    const day = days()[dayIndex]
    if (!day) return ''
    const date = new Date(`${day}T12:00:00Z`)
    if (Number.isNaN(date.getTime())) return day
    return new Intl.DateTimeFormat(locale.value, { weekday: style, timeZone: 'UTC' }).format(date)
  }

  function hour(minute: number): string {
    const normalized = ((Math.round(minute) % 1440) + 1440) % 1440
    const date = new Date(Date.UTC(2026, 0, 5, Math.floor(normalized / 60), normalized % 60))
    return new Intl.DateTimeFormat(locale.value, { hour: 'numeric', timeZone: 'UTC' }).format(date)
  }

  // "09:15"-style clock time for tooltips, in the user's locale.
  function clock(minute: number): string {
    const normalized = ((Math.round(minute) % 1440) + 1440) % 1440
    const date = new Date(Date.UTC(2026, 0, 5, Math.floor(normalized / 60), normalized % 60))
    return new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit', timeZone: 'UTC' }).format(date)
  }

  function category(name: string): string {
    return name === WEEKLY_OTHER_KEY ? t('weekly.charts.otherCategory') : categoryLabel(name, t)
  }

  function app(key: string, name: string): string {
    return key === WEEKLY_OTHER_KEY || name === '' ? t('weekly.charts.otherApp') : name
  }

  return { weekday, hour, clock, category, app }
}
