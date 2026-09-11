import type {
  CapabilitiesDTO,
  DayContextDTO,
  TimelineDayDTO,
} from '@/api/dto'

interface TimelineBackend {
  GetCapabilities?: () => Promise<CapabilitiesDTO>
  GetDayContext?: (day: string) => Promise<DayContextDTO>
  GetTimelineDay?: (day: string) => Promise<TimelineDayDTO>
}

interface WailsRuntime {
  EventsOnMultiple?: (
    eventName: string,
    callback: (...data: unknown[]) => void,
    maxCallbacks: number,
  ) => () => void
}

type WailsWindow = Window & {
  go?: { app?: { Backend?: TimelineBackend } }
  runtime?: WailsRuntime
}

export const TIMELINE_UNAVAILABLE = 'timeline_unavailable'

export class TimelineUnavailableError extends Error {
  constructor() {
    super(TIMELINE_UNAVAILABLE)
    this.name = 'TimelineUnavailableError'
  }
}

function backend(): TimelineBackend | null {
  return (window as WailsWindow).go?.app?.Backend ?? null
}

export function hasDayContextBinding(): boolean {
  return typeof backend()?.GetDayContext === 'function'
}

export async function getDayContext(day = ''): Promise<DayContextDTO> {
  const method = backend()?.GetDayContext
  if (typeof method !== 'function') throw new TimelineUnavailableError()
  return method(day)
}

export async function getTimelineDay(day: string): Promise<TimelineDayDTO> {
  const method = backend()?.GetTimelineDay
  if (typeof method !== 'function') throw new TimelineUnavailableError()
  return method(day)
}

export async function getTimelineCapabilities(): Promise<CapabilitiesDTO | null> {
  const method = backend()?.GetCapabilities
  return typeof method === 'function' ? method() : null
}

/**
 * Subscribe without importing generated runtime code into browser previews.
 * Wails owns the returned disposer; a missing runtime is a valid no-op state.
 */
export function onTimelineUpdated(callback: (day: string | null) => void): () => void {
  const method = (window as WailsWindow).runtime?.EventsOnMultiple
  if (typeof method !== 'function') return () => undefined

  return method(
    'timeline:updated',
    (raw: unknown) => {
      if (typeof raw !== 'object' || raw === null) return callback(null)
      const day = (raw as Record<string, unknown>).day
      callback(typeof day === 'string' ? day : null)
    },
    -1,
  )
}
