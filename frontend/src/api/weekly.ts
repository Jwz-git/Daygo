import type { WeeklyDashboardDTO } from '@/api/dto'

interface WeeklyBackend {
  GetWeeklyDashboard?: (weekStart: string) => Promise<WeeklyDashboardDTO>
}

type WeeklyWindow = Window & {
  go?: { app?: { Backend?: WeeklyBackend } }
}

export class WeeklyUnavailableError extends Error {
  constructor() {
    super('weekly_unavailable')
    this.name = 'WeeklyUnavailableError'
  }
}

function backend(): WeeklyBackend | null {
  return (window as WeeklyWindow).go?.app?.Backend ?? null
}

export function hasWeeklyBinding(): boolean {
  return typeof backend()?.GetWeeklyDashboard === 'function'
}

export async function getWeeklyDashboard(weekStart = ''): Promise<WeeklyDashboardDTO> {
  const method = backend()?.GetWeeklyDashboard
  if (typeof method !== 'function') throw new WeeklyUnavailableError()
  return method(weekStart)
}
