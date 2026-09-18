import type { RangeDTO, TimelineCardDTO, TimelineDayDTO } from '@/api/dto'
import { appSiteValues, preferredAppSite } from '@/lib/appSiteIcon'
import { categoryLabel as translateCategory } from '@/lib/categoryLabel'
import { boxesOverlap, cardIntersectsRanges, positionRange, safeCategoryColor, uncoveredBy } from './layout'

/*
 * Week-grid layout, kept as pure functions so the column math is unit-testable
 * without mounting the component. Each column derives from its own day's
 * 04:00→04:00 window (GetTimelineDay), so a card lands in the column of the
 * logical day it belongs to; the frontend never recomputes that boundary.
 *
 * Unlike the day track the week grid is taller than the viewport and scrolls:
 * columns use a dedicated pixels-per-minute scale instead of percent-of-view.
 */

/** Slightly denser than the day track: seven columns must stay readable. */
export const WEEK_PIXELS_PER_MINUTE = 1.5
export const MIN_WEEK_TRACK_HEIGHT = 1500

/*
 * Lines of title text that fit a week card of the given height. The card is
 * border-box with 1px borders top/bottom plus 8px padding on each side, so
 * only height - 18 is text space; 11px at 1.4 line-height is ~15.4px/line.
 * Under-subtracting here is what cut the last line in half instead of
 * ellipsizing it.
 */
export function weekCardClampLines(height: number): number {
  return Math.max(1, Math.floor((height - 14) / 15.4))
}

export interface WeekCard {
  id: number
  /** The full backend card, so the inspector can show it without refetching. */
  card: TimelineCardDTO
  top: number
  height: number
  title: string
  color: string
  category: string
  isIdle: boolean
  /** Lines of title text that fit the card; extra lines ellipsize. */
  clampLines: number
  /** First app/site of the card, for the leading icon. */
  site: string | null
  /** Candidate apps/sites of the card, for icon resolution with fallback. */
  sites: string[]
  /** A batch covering this card is being analyzed again; the card is stale. */
  regenerating: boolean
}

export interface WeekHourMark {
  top: number
  label: string
}

export interface WeekColumn {
  day: string
  height: number
  /** The column's own 04:00→04:00 window, for now-line placements. */
  windowStartTs: number
  windowEndTs: number
  cards: WeekCard[]
  processing: Array<{ top: number; height: number }>
  hourMarks: WeekHourMark[]
  hasData: boolean
}

function columnHeight(day: TimelineDayDTO): number {
  const minutes = (day.dayEndTs - day.dayStartTs) / 60
  return Math.max(MIN_WEEK_TRACK_HEIGHT, minutes * WEEK_PIXELS_PER_MINUTE)
}

function hourMarksFor(day: TimelineDayDTO, height: number, format: Intl.DateTimeFormat): WeekHourMark[] {
  const span = day.dayEndTs - day.dayStartTs
  if (span <= 0) return []
  const marks: WeekHourMark[] = []
  const step = 60 * 60
  for (let offset = 0; offset <= span; offset += step) {
    const ts = day.dayStartTs + offset
    marks.push({
      top: (offset / span) * height,
      label: format.format(new Date(ts * 1000)),
    })
  }
  return marks
}

function placedRanges(ranges: RangeDTO[], day: TimelineDayDTO, height: number): Array<{ top: number; height: number }> {
  return ranges.map((range) => positionRange(range.startTs, range.endTs, day.dayStartTs, day.dayEndTs, height, 22))
}

export function buildWeekColumns(
  days: readonly (TimelineDayDTO | null)[],
  filterCategory: string | null,
  format: Intl.DateTimeFormat,
): WeekColumn[] {
  const colorOf = new Map<string, string>()
  for (const day of days) {
    if (day === null) continue
    for (const category of day.categories) colorOf.set(category.name, safeCategoryColor(category.colorHex))
  }

  return days.map((day) => {
    if (day === null) {
      return {
        day: '',
        height: MIN_WEEK_TRACK_HEIGHT,
        windowStartTs: 0,
        windowEndTs: 0,
        cards: [],
        processing: [],
        hourMarks: [],
        hasData: false,
      }
    }
    const height = columnHeight(day)
    const boxes = day.cards
      .filter((card) => {
        if (card.category === 'System') return false
        return filterCategory === null || card.category === filterCategory
      })
      .map((card) => ({
        card,
        ...positionRange(card.startTs, card.endTs, day.dayStartTs, day.dayEndTs, height, 34),
      }))
    const processing = placedRanges(day.processingRanges, day, height)
    return {
      day: day.day,
      height,
      windowStartTs: day.dayStartTs,
      windowEndTs: day.dayEndTs,
      cards: boxes.map((box) => ({
        id: box.card.id,
        card: box.card,
        top: box.top,
        height: box.height,
        title: box.card.title,
        color: colorOf.get(box.card.category) ?? safeCategoryColor(undefined),
        category: translateCategory(box.card.category, () => box.card.category),
        isIdle: box.card.isIdle,
        clampLines: weekCardClampLines(box.height),
        site: preferredAppSite(box.card.appSites ?? null),
        sites: appSiteValues(box.card.appSites ?? null),
        regenerating: cardIntersectsRanges(box.card, day.processingRanges),
      })),
      // A window belongs to the card that already covers it: the block is the
      // stand-in for a gap, so it yields wherever a card exists.
      processing: uncoveredBy(processing, boxes),
      hourMarks: hourMarksFor(day, height, format),
      hasData: true,
    }
  })
}
