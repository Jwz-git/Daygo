import type { CapabilitiesDTO, DayContextDTO, TimelineDayDTO } from '@/api/dto'

export interface TimelineDevelopmentFixture {
  context: DayContextDTO
  day: TimelineDayDTO
  capabilities: CapabilitiesDTO
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isTimelineFixture(value: unknown): value is TimelineDevelopmentFixture {
  if (!isRecord(value) || !isRecord(value.context) || !isRecord(value.day)) return false
  if (!isRecord(value.capabilities)) return false

  return (
    typeof value.context.day === 'string' &&
    typeof value.context.dayStartTs === 'number' &&
    typeof value.context.dayEndTs === 'number' &&
    Array.isArray(value.day.cards) &&
    Array.isArray(value.day.categories) &&
    Array.isArray(value.day.failures) &&
    Array.isArray(value.day.processingRanges) &&
    typeof value.capabilities.canWrite === 'boolean'
  )
}

/**
 * Browser/Wails development fallback only. Vite serves the payload from
 * frontend/dev-fixtures; that directory is outside src and never enters the
 * production bundle. A malformed or missing fixture quietly preserves the
 * normal "capability unavailable" state.
 */
export async function getTimelineDevelopmentFixture(): Promise<TimelineDevelopmentFixture | null> {
  if (!import.meta.env.DEV) return null

  try {
    const response = await fetch('/__daygo_dev__/timeline', { cache: 'no-store' })
    if (!response.ok) return null
    const raw: unknown = await response.json()
    return isTimelineFixture(raw) ? raw : null
  } catch {
    return null
  }
}
