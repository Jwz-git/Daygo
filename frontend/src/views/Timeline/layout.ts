export const MIN_CARD_HEIGHT = 38
export const PIXELS_PER_MINUTE = 1.05
export const MIN_TRACK_HEIGHT = 960

export interface PositionedRange {
  top: number
  height: number
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

export function safeCategoryColor(color: string | undefined): string {
  return typeof color === 'string' && /^#[0-9a-f]{6}$/i.test(color)
    ? color
    : 'var(--dg-accent)'
}
