import type {
  CapabilitiesDTO,
  DailyRecapDTO,
  DayContextDTO,
  DayGoalDTO,
  JournalDayDTO,
  TimelineDayDTO,
} from '@/api/dto'

interface DailyBackend {
  GetCapabilities?: () => Promise<CapabilitiesDTO>
  GetDayContext?: (day: string) => Promise<DayContextDTO>
  GetTimelineDay?: (day: string) => Promise<TimelineDayDTO>
  GetDailyRecap?: (standupDay: string) => Promise<DailyRecapDTO>
  GetJournalDay?: (day: string) => Promise<JournalDayDTO>
  SaveJournalDay?: (entry: JournalDayDTO) => Promise<void>
  GetDayGoal?: (day: string) => Promise<DayGoalDTO>
  SaveDayGoal?: (goal: DayGoalDTO) => Promise<void>
}

interface WailsRuntime {
  EventsOnMultiple?: (
    eventName: string,
    callback: (...data: unknown[]) => void,
    maxCallbacks: number,
  ) => () => void
}

type DailyWindow = Window & {
  go?: { app?: { Backend?: DailyBackend } }
  runtime?: WailsRuntime
}

export class DailyUnavailableError extends Error {
  constructor() {
    super('daily_unavailable')
    this.name = 'DailyUnavailableError'
  }
}

function backend(): DailyBackend | null {
  return (window as DailyWindow).go?.app?.Backend ?? null
}

export function hasDailyDayBinding(): boolean {
  const current = backend()
  return (
    typeof current?.GetDayContext === 'function' &&
    typeof current.GetTimelineDay === 'function'
  )
}

export function hasDailyRecapBinding(): boolean {
  return typeof backend()?.GetDailyRecap === 'function'
}

export async function getDailyContext(day = ''): Promise<DayContextDTO> {
  const method = backend()?.GetDayContext
  if (typeof method !== 'function') throw new DailyUnavailableError()
  return method(day)
}

export async function getDailyTimeline(day: string): Promise<TimelineDayDTO> {
  const method = backend()?.GetTimelineDay
  if (typeof method !== 'function') throw new DailyUnavailableError()
  return method(day)
}

export async function getDailyRecap(standupDay: string): Promise<DailyRecapDTO> {
  const method = backend()?.GetDailyRecap
  if (typeof method !== 'function') throw new DailyUnavailableError()
  return method(standupDay)
}

export async function getDailyCapabilities(): Promise<CapabilitiesDTO | null> {
  const method = backend()?.GetCapabilities
  return typeof method === 'function' ? method() : null
}

export function hasJournalBinding(): boolean {
  const current = backend()
  return (
    typeof current?.GetJournalDay === 'function' &&
    typeof current.SaveJournalDay === 'function'
  )
}

export function hasGoalBinding(): boolean {
  const current = backend()
  return (
    typeof current?.GetDayGoal === 'function' &&
    typeof current.SaveDayGoal === 'function'
  )
}

export async function getJournalDay(day: string): Promise<JournalDayDTO> {
  const method = backend()?.GetJournalDay
  if (typeof method !== 'function') throw new DailyUnavailableError()
  return method(day)
}

export async function saveJournalDay(entry: JournalDayDTO): Promise<void> {
  const method = backend()?.SaveJournalDay
  if (typeof method !== 'function') throw new DailyUnavailableError()
  return method(entry)
}

export async function getDayGoal(day: string): Promise<DayGoalDTO> {
  const method = backend()?.GetDayGoal
  if (typeof method !== 'function') throw new DailyUnavailableError()
  return method(day)
}

export async function saveDayGoal(goal: DayGoalDTO): Promise<void> {
  const method = backend()?.SaveDayGoal
  if (typeof method !== 'function') throw new DailyUnavailableError()
  return method(goal)
}

/**
 * Subscribe without importing generated runtime code into browser previews.
 * Payloads are invalidation-only: {day}. A missing runtime is a valid no-op.
 */
function onDailyInvalidated(
  eventName: string,
  callback: (day: string | null) => void,
): () => void {
  const method = (window as DailyWindow).runtime?.EventsOnMultiple
  if (typeof method !== 'function') return () => undefined
  return method(eventName, (raw: unknown) => {
    if (typeof raw !== 'object' || raw === null) return callback(null)
    const day = (raw as Record<string, unknown>).day
    callback(typeof day === 'string' ? day : null)
  }, -1)
}

export function onJournalUpdated(callback: (day: string | null) => void): () => void {
  return onDailyInvalidated('journal:updated', callback)
}

export function onGoalUpdated(callback: (day: string | null) => void): () => void {
  return onDailyInvalidated('goal:updated', callback)
}
