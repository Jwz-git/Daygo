import type { WeeklyDashboardDTO } from '@/api/dto'

export interface WeeklyDistribution {
  categories: { name: string; minutes: number; colorHex: string }[]
}

/** The distribution card uses category totals; other charts use weeklyCharts. */
export function buildWeeklyDistribution(dashboard: WeeklyDashboardDTO): WeeklyDistribution {
  return {
    categories: dashboard.categories
      .map(({ name, minutes, colorHex }) => ({
        name: name.trim(),
        minutes: Number.isFinite(minutes) ? Math.max(0, minutes) : 0,
        // Match the daily view's neutral fallback for uncoloured categories.
        colorHex: /^#[0-9a-f]{6}$/i.test(colorHex) ? colorHex : '#7D7A84',
      }))
      .filter((category) => category.name.length > 0 && category.minutes > 0)
      .sort((left, right) => right.minutes - left.minutes || left.name.localeCompare(right.name)),
  }
}
