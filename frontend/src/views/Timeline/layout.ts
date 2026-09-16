// 2.6px per minute: one 15-minute card (the default batch window) gets 39px
// of vertical space — a single-line card with real breathing room between
// neighbours. Shorter cards fall back to MIN_CARD_HEIGHT and lane-pack.
export const MIN_CARD_HEIGHT = 34
export const PIXELS_PER_MINUTE = 2.6
export const MIN_TRACK_HEIGHT = 960

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
  laneIndex: number
  laneCount: number
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

/**
 * Put display boxes that overlap because of the minimum hit target into lanes.
 * Their vertical position remains tied to the backend timestamps; only the
 * horizontal space is shared, so a four-minute card cannot cover its neighbour.
 */
export function layoutTimelineCards(
  cards: readonly TimelineLayoutInput[],
  dayStartTs: number,
  dayEndTs: number,
  height: number,
): PositionedCard[] {
  const placed = cards
    .map((card) => ({
      id: card.id,
      ...positionRange(card.startTs, card.endTs, dayStartTs, dayEndTs, height, MIN_CARD_HEIGHT),
      laneIndex: 0,
      laneCount: 1,
    }))
    .sort((left, right) => left.top - right.top || left.id - right.id)

  let cluster: PositionedCard[] = []
  let clusterBottom = -Infinity

  const finishCluster = () => {
    if (cluster.length === 0) return
    const laneBottoms: number[] = []

    for (const card of cluster) {
      let laneIndex = laneBottoms.findIndex((bottom) => bottom <= card.top)
      if (laneIndex === -1) {
        laneIndex = laneBottoms.length
        laneBottoms.push(card.top + card.height)
      } else {
        laneBottoms[laneIndex] = card.top + card.height
      }
      card.laneIndex = laneIndex
    }

    const laneCount = Math.max(1, laneBottoms.length)
    for (const card of cluster) card.laneCount = laneCount
    cluster = []
    clusterBottom = -Infinity
  }

  for (const card of placed) {
    // Sub-pixel arithmetic: a 15-minute card at 2.6px/min can land
    // 0.0000001px into its predecessor's slot; that rounding noise must not
    // trigger lane packing.
    if (cluster.length > 0 && card.top >= clusterBottom - 0.5) finishCluster()
    cluster.push(card)
    clusterBottom = Math.max(clusterBottom, card.top + card.height)
  }
  finishCluster()

  return placed
}

export function safeCategoryColor(color: string | undefined): string {
  return typeof color === 'string' && /^#[0-9a-f]{6}$/i.test(color)
    ? color
    : 'var(--dg-accent)'
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
