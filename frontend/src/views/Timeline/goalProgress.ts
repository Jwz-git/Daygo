import type { CategoryDTO, DayGoalDTO, TimelineCardDTO } from '@/api/dto'

/*
 * Day-goal progress after Dayflow's DaySummaryStats: focus is the minutes of
 * the day's cards in the goal's focus categories (one segment per category,
 * in the goal's order), distraction the minutes in its distraction
 * categories. System cards never count. A goal category is matched by its
 * current name (resolved through its id), falling back to the name saved
 * with the goal when the category has since been removed.
 */
export interface GoalProgressSegment {
  id: string
  name: string
  colorHex: string
  minutes: number
}

export interface GoalProgress {
  focusMinutes: number
  focusSegments: GoalProgressSegment[]
  distractionMinutes: number
}

type GoalCard = Pick<TimelineCardDTO, 'category' | 'durationMinutes'>

const normalized = (name: string): string => name.trim().toLowerCase()

export function goalProgress(
  goal: Pick<DayGoalDTO, 'focusCategories' | 'distractionCategories'>,
  categories: readonly CategoryDTO[],
  cards: readonly GoalCard[],
): GoalProgress {
  const systemNames = new Set(
    categories.filter((category) => category.isSystem).map((category) => normalized(category.name)),
  )
  systemNames.add('system')

  const minutesByName = new Map<string, number>()
  for (const card of cards) {
    const name = normalized(card.category)
    if (systemNames.has(name)) continue
    minutesByName.set(name, (minutesByName.get(name) ?? 0) + Math.max(0, card.durationMinutes))
  }

  const resolve = (ref: DayGoalDTO['focusCategories'][number]) => {
    const current = categories.find((category) => category.id === ref.categoryId)
    return {
      id: ref.categoryId,
      name: current?.name ?? ref.name,
      colorHex: current?.colorHex ?? ref.colorHex,
    }
  }

  const focusSegments = goal.focusCategories.map((ref) => {
    const category = resolve(ref)
    return { ...category, minutes: minutesByName.get(normalized(category.name)) ?? 0 }
  })
  const distractionMinutes = goal.distractionCategories.reduce(
    (sum, ref) => sum + (minutesByName.get(normalized(resolve(ref).name)) ?? 0),
    0,
  )
  return {
    focusMinutes: focusSegments.reduce((sum, segment) => sum + segment.minutes, 0),
    focusSegments,
    distractionMinutes,
  }
}

/** Dayflow's compact hours: "2", "4.5". */
export function compactHours(minutes: number): string {
  const hours = minutes / 60
  return Math.abs(Math.round(hours) - hours) < 0.01 ? String(Math.round(hours)) : hours.toFixed(1)
}

/** Dayflow's default plan (DayGoalPlan.defaultPlan): 4.5 h focus, 2 h distraction. */
export const DEFAULT_FOCUS_TARGET_MINUTES = 270
export const DEFAULT_DISTRACTION_LIMIT_MINUTES = 120

const isDistractionName = (name: string): boolean => {
  const key = normalized(name)
  return key === 'distraction' || key === 'distractions'
}

/*
 * The categories a day with no goal yet starts from, as in Dayflow: every
 * user category (not system, not idle) in sort order is focus, except the
 * Distraction category, which is the distraction set.
 */
export function defaultGoalCategories(categories: readonly CategoryDTO[]): {
  focus: string[]
  distraction: string[]
} {
  const selectable = categories
    .filter((category) => !category.isSystem && !category.isIdle)
    .sort((a, b) => a.sortOrder - b.sortOrder)
  return {
    focus: selectable.filter((category) => !isDistractionName(category.name)).map((category) => category.id),
    distraction: selectable.filter((category) => isDistractionName(category.name)).map((category) => category.id),
  }
}
