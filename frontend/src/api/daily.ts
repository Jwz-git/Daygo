import type {
  CapabilitiesDTO,
  DailyRecapDTO,
  DayContextDTO,
  TimelineDayDTO,
} from '@/api/dto'

interface DailyBackend {
  GetCapabilities?: () => Promise<CapabilitiesDTO>
  GetDayContext?: (day: string) => Promise<DayContextDTO>
  GetTimelineDay?: (day: string) => Promise<TimelineDayDTO>
  GetDailyRecap?: (standupDay: string) => Promise<DailyRecapDTO>
}

type DailyWindow = Window & {
  go?: { app?: { Backend?: DailyBackend } }
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
