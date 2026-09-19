import type {
  CapabilitiesDTO,
  CategoryDTO,
  DayContextDTO,
  TimelineDayDTO,
} from '@/api/dto'

interface TimelineBackend {
  SaveCategories?: (categories: CategoryDTO[]) => Promise<void>
  GetCapabilities?: () => Promise<CapabilitiesDTO>
  GetDayContext?: (day: string) => Promise<DayContextDTO>
  GetTimelineDay?: (day: string) => Promise<TimelineDayDTO>
  UpdateCardCategory?: (cardID: number, category: string) => Promise<void>
  UpdateCardTitle?: (cardID: number, title: string) => Promise<void>
  UpdateCardSummary?: (cardID: number, text: string) => Promise<void>
  UpdateCardDetailedSummary?: (cardID: number, text: string) => Promise<void>
  DeleteCard?: (cardID: number) => Promise<void>
  RetryBatches?: (batchIDs: number[]) => Promise<void>
  ReprocessDay?: (day: string) => Promise<void>
  ReprocessCard?: (cardID: number) => Promise<void>
  DeleteBatches?: (batchIDs: number[]) => Promise<void>
  ClearHistoryData?: () => Promise<void>
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
  updateSummary: boolean
  updateDetailedSummary: boolean
  deleteCard: boolean
  retryBatches: boolean
  reprocessDay: boolean
  reprocessCard: boolean
  deleteBatches: boolean
  clearHistory: boolean
  manageCategories: boolean
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
    updateSummary: typeof current?.UpdateCardSummary === 'function',
    updateDetailedSummary: typeof current?.UpdateCardDetailedSummary === 'function',
    deleteCard: typeof current?.DeleteCard === 'function',
    retryBatches: typeof current?.RetryBatches === 'function',
    reprocessDay: typeof current?.ReprocessDay === 'function',
    reprocessCard: typeof current?.ReprocessCard === 'function',
    deleteBatches: typeof current?.DeleteBatches === 'function',
    clearHistory: typeof current?.ClearHistoryData === 'function',
    // manageCategories is handled by the UI; no backend binding needed
    manageCategories: false,
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

export async function saveCategories(categories: CategoryDTO[]): Promise<void> {
  return requiredMethod('SaveCategories')(categories)
}

export async function updateCardCategory(cardID: number, category: string): Promise<void> {
  return requiredMethod('UpdateCardCategory')(cardID, category)
}

export async function updateCardTitle(cardID: number, title: string): Promise<void> {
  return requiredMethod('UpdateCardTitle')(cardID, title)
}

export async function updateCardSummary(cardID: number, text: string): Promise<void> {
  return requiredMethod('UpdateCardSummary')(cardID, text)
}

export async function updateCardDetailedSummary(cardID: number, text: string): Promise<void> {
  return requiredMethod('UpdateCardDetailedSummary')(cardID, text)
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

export async function reprocessCard(cardID: number): Promise<void> {
  return requiredMethod('ReprocessCard')(cardID)
}

export async function deleteBatches(batchIDs: number[]): Promise<void> {
  return requiredMethod('DeleteBatches')(batchIDs)
}

export async function clearHistoryData(): Promise<void> {
  return requiredMethod('ClearHistoryData')()
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
