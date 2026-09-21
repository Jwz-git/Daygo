// 2.6px per minute: one 15-minute card (the default batch window) gets 39px
// of vertical space — a single-line card with real breathing room between
// neighbours.
export const PIXELS_PER_MINUTE = 2.6
export const MIN_TRACK_HEIGHT = 960

/*
 * The day track mirrors the reference canvas timeline (legacy/Dayflow
 * CanvasTimelineDataView): a card is exactly as tall as its own minutes, minus
 * a 2px gap split between its top and bottom, and never shorter than 10px so a
 * two-minute activity still paints a clickable bar. Cards are NOT split into
 * side-by-side lanes — an overlap in the data is trimmed away in time instead
 * (see resolveDisplaySpans), so every row stays full width.
 */
export const MIN_CARD_HEIGHT = 10
export const CARD_GAP = 2

/*
 * Text rules, also from the reference: below ten minutes a row is a bare bar
 * (the title would have to shrink past legibility to fit), and below thirteen
 * minutes it keeps a tight, vertically centred line instead of the roomy one.
 */
export const CARD_TEXT_MINUTES = 10
export const CARD_COMPACT_MINUTES = 13

/** True when a row is tall enough to carry its title, icon and time label. */
export function cardShowsText(minutes: number): boolean {
  return minutes >= CARD_TEXT_MINUTES
}

/** True when a row takes the tight, centred single-line layout. */
export function cardIsCompact(minutes: number): boolean {
  return minutes < CARD_COMPACT_MINUTES
}

export interface PositionedRange {
  top: number
  height: number
}

export interface TimelineLayoutInput {
  id: number
  startTs: number
  endTs: number
}

export interface PositionedCard extends PositionedRange {
  id: number
  /** Minutes actually drawn, after any overlap trim; drives the text rules. */
  minutes: number
}

export interface DisplaySpan {
  id: number
  startTs: number
  endTs: number
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

export function trackHeight(dayStartTs: number, dayEndTs: number): number {
  const minutes = Math.max(0, dayEndTs - dayStartTs) / 60
  return Math.max(MIN_TRACK_HEIGHT, minutes * PIXELS_PER_MINUTE)
}

export function positionRange(
  startTs: number,
  endTs: number,
  dayStartTs: number,
  dayEndTs: number,
  height: number,
  minimumHeight = 2,
): PositionedRange {
  const duration = Math.max(1, dayEndTs - dayStartTs)
  const start = clamp(startTs, dayStartTs, dayEndTs)
  const end = clamp(Math.max(startTs, endTs), dayStartTs, dayEndTs)
  const top = ((start - dayStartTs) / duration) * height
  const naturalHeight = ((end - start) / duration) * height
  return {
    top,
    height: Math.min(height - top, Math.max(minimumHeight, naturalHeight)),
  }
}

/** The card's own box: its minutes of track, inset by the gap, floored at 10px. */
export function cardBox(
  startTs: number,
  endTs: number,
  dayStartTs: number,
  dayEndTs: number,
  height: number,
): PositionedRange {
  const slot = positionRange(startTs, endTs, dayStartTs, dayEndTs, height, 0)
  return {
    top: slot.top + CARD_GAP / 2,
    height: Math.max(MIN_CARD_HEIGHT, slot.height - CARD_GAP),
  }
}

/*
 * Overlapping cards are an upstream data bug, not a layout style. The reference
 * resolves them by trimming the longer card back to the shorter one's edge —
 * display only, stored data untouched — so the row stays full width instead of
 * splitting into side-by-side lanes. A card trimmed down to nothing disappears;
 * a duplicated card (identical span) leaves one row.
 */
export function resolveDisplaySpans(cards: readonly TimelineLayoutInput[]): DisplaySpan[] {
  const spans: DisplaySpan[] = cards
    .map((card) => ({ id: card.id, startTs: card.startTs, endTs: card.endTs }))
    .sort((left, right) => left.startTs - right.startTs || left.endTs - right.endTs)

  const maxPasses = 8
  for (let pass = 0; pass < maxPasses; pass++) {
    let changed = false
    // Every trim is followed by a fresh scan: a shortened card can overlap
    // something the current walk has already passed.
    scan: for (let i = 0; i < spans.length; i++) {
      for (let j = i + 1; j < spans.length; j++) {
        if (spans[j].startTs >= spans[i].endTs) break
        const first = spans[i]
        const second = spans[j]
        if (Math.min(first.endTs, second.endTs) <= Math.max(first.startTs, second.startTs)) {
          continue
        }

        // The shorter card wins the shared minutes; ties keep the earlier one.
        const firstMinutes = first.endTs - first.startTs
        const secondMinutes = second.endTs - second.startTs
        const smallIndex = firstMinutes <= secondMinutes ? i : j
        const largeIndex = smallIndex === i ? j : i
        const smaller = spans[smallIndex]
        const larger = spans[largeIndex]

        if (larger.startTs < smaller.startTs && smaller.endTs < larger.endTs) {
          // The shorter card sits inside the longer one: cut it out of the
          // longer card's larger side.
          if (larger.endTs - smaller.endTs >= smaller.startTs - larger.startTs) {
            larger.startTs = smaller.endTs
          } else {
            larger.endTs = smaller.startTs
          }
        } else if (smaller.startTs <= larger.startTs && larger.startTs < smaller.endTs) {
          larger.startTs = smaller.endTs
        } else if (smaller.startTs < larger.endTs && larger.endTs <= smaller.endTs) {
          larger.endTs = smaller.startTs
        }

        if (larger.endTs <= larger.startTs) {
          spans.splice(largeIndex, 1)
        } else {
          // Trimming a start can move the card past its successor.
          spans.sort((left, right) => left.startTs - right.startTs || left.endTs - right.endTs)
        }
        changed = true
        continue scan
      }
    }
    if (!changed) break
  }

  return spans
}

/**
 * Position every card of the day as one full-width row: no lanes, height tied
 * to the card's own minutes, and the top edge on its own timestamp.
 */
export function layoutTimelineCards(
  cards: readonly TimelineLayoutInput[],
  dayStartTs: number,
  dayEndTs: number,
  height: number,
): PositionedCard[] {
  return resolveDisplaySpans(cards).map((span) => ({
    id: span.id,
    minutes: (span.endTs - span.startTs) / 60,
    ...cardBox(span.startTs, span.endTs, dayStartTs, dayEndTs, height),
  }))
}

// Neutral grey for a category whose colour is missing or malformed (e.g. a
// deleted category a card still references). Kept in sync with the daily and
// weekly stores' safeColor fallback so the same category reads identically
// across timeline, daily and weekly instead of drifting to the accent hue.
export const FALLBACK_CATEGORY_COLOR = '#7D7A84'

export function safeCategoryColor(color: string | undefined): string {
  return typeof color === 'string' && /^#[0-9a-f]{6}$/i.test(color)
    ? color
    : FALLBACK_CATEGORY_COLOR
}

/**
 * True when two drawn boxes share vertical space. Neighbours that merely touch
 * do not: the comparison is on the rendered geometry, so the minimum hit
 * height counts exactly as much as the timestamps do.
 */
export function boxesOverlap(a: PositionedRange, b: PositionedRange): boolean {
  return a.top < b.top + b.height && b.top < a.top + a.height
}

/** The boxes from `boxes` that at least one box in `occupied` overlaps. */
export function coveredBy<T extends PositionedRange>(
  boxes: readonly T[],
  occupied: readonly PositionedRange[],
): T[] {
  return boxes.filter((box) => occupied.some((other) => boxesOverlap(box, other)))
}

/** The boxes from `boxes` that no box in `occupied` overlaps. */
export function uncoveredBy<T extends PositionedRange>(
  boxes: readonly T[],
  occupied: readonly PositionedRange[],
): T[] {
  return boxes.filter((box) => !occupied.some((other) => boxesOverlap(box, other)))
}

/**
 * True when a card's actual time span intersects a processing range by at least 1 second.
 * Using temporal intersection instead of rendered pixel boxes keeps the minimum
 * heights (a card's floor, a block's PROCESSING_MIN_HEIGHT) from bleeding into
 * and colouring adjacent cards above or below.
 */
export function cardIntersectsRanges(
  card: { startTs: number; endTs: number },
  ranges: readonly { startTs: number; endTs: number }[],
): boolean {
  return ranges.some(
    (range) => Math.min(card.endTs, range.endTs) > Math.max(card.startTs, range.startTs),
  )
}

/**
 * True when a running batch will replace this card. A batch rewrites the cards
 * it produced, and its rewrite spans further back than its window: an ongoing
 * pass starts at the card it continues, so a card it owns can sit entirely
 * before the window and would otherwise show no sign of being regenerated while
 * its neighbour below does. Intersection still counts — a neighbouring batch
 * that reaches into this card replaces it too.
 */
export function isRegenerating(
  card: { batchId?: number | null; startTs: number; endTs: number },
  ranges: readonly { startTs: number; endTs: number; batchIds?: readonly number[] }[],
): boolean {
  return ranges.some(
    (range) =>
      (card.batchId !== null && card.batchId !== undefined && (range.batchIds ?? []).includes(card.batchId)) ||
      Math.min(card.endTs, range.endTs) > Math.max(card.startTs, range.startTs),
  )
}
