import type {
  CapabilitiesDTO,
  DayContextDTO,
  TimelineDayDTO,
} from '@/api/dto'

interface TimelineBackend {
  GetCapabilities?: () => Promise<CapabilitiesDTO>
  GetDayContext?: (day: string) => Promise<DayContextDTO>
  GetTimelineDay?: (day: string) => Promise<TimelineDayDTO>
  UpdateCardCategory?: (cardID: number, category: string) => Promise<void>
  UpdateCardTitle?: (cardID: number, title: string) => Promise<void>
  DeleteCard?: (cardID: number) => Promise<void>
  RetryBatches?: (batchIDs: number[]) => Promise<void>
  ReprocessDay?: (day: string) => Promise<void>
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

export interface TimelineActionAvailability {
  updateCategory: boolean
  updateTitle: boolean
  deleteCard: boolean
  retryBatches: boolean
  reprocessDay: boolean
}

function backend(): TimelineBackend | null {
  return (window as WailsWindow).go?.app?.Backend ?? null
}

export function hasDayContextBinding(): boolean {
  return typeof backend()?.GetDayContext === 'function'
}

export function hasTimelineDayBinding(): boolean {
  const current = backend()
  return (
    typeof current?.GetDayContext === 'function' &&
    typeof current.GetTimelineDay === 'function'
  )
}

export function getTimelineActionAvailability(): TimelineActionAvailability {
  const current = backend()
  return {
    updateCategory: typeof current?.UpdateCardCategory === 'function',
    updateTitle: typeof current?.UpdateCardTitle === 'function',
    deleteCard: typeof current?.DeleteCard === 'function',
    retryBatches: typeof current?.RetryBatches === 'function',
    reprocessDay: typeof current?.ReprocessDay === 'function',
  }
}

function requiredMethod<K extends keyof TimelineBackend>(name: K): NonNullable<TimelineBackend[K]> {
  const method = backend()?.[name]
  if (typeof method !== 'function') throw new TimelineUnavailableError()
  return method as NonNullable<TimelineBackend[K]>
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

export async function updateCardCategory(cardID: number, category: string): Promise<void> {
  return requiredMethod('UpdateCardCategory')(cardID, category)
}

export async function updateCardTitle(cardID: number, title: string): Promise<void> {
  return requiredMethod('UpdateCardTitle')(cardID, title)
}

export async function deleteCard(cardID: number): Promise<void> {
  return requiredMethod('DeleteCard')(cardID)
}

export async function retryBatches(batchIDs: number[]): Promise<void> {
  return requiredMethod('RetryBatches')(batchIDs)
}

export async function reprocessDay(day: string): Promise<void> {
  return requiredMethod('ReprocessDay')(day)
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
