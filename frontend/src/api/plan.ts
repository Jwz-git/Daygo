import { normalizePlanDay } from '@/api/normalizeDTO'
import type { GeneratedBindings } from '@/api/generatedBindings'
import type { PlanBlockDTO, PlanBlockInputDTO, PlanBlockStatus, PlanDayDTO } from '@/api/dto'
import { canUseDevelopmentTestData } from '@/api/developmentFixtures'

/*
 * Thin wrapper over the plan bindings (docs/05 §5.5.2). Times are HH:mm plus
 * the logical day; Go resolves them to instants, the frontend never does.
 * In a browser preview with test data on, an in-memory stand-in keeps the
 * panel exercisable; it never runs in production.
 */

type PlanBackend = GeneratedBindings<
  | 'GetPlanDay'
  | 'SavePlanBlock'
  | 'SetPlanBlockStatus'
  | 'DeletePlanBlock'
>

interface WailsRuntime {
  EventsOnMultiple?: (eventName: string, callback: (...data: unknown[]) => void, maxCallbacks: number) => () => void
}

type PlanWindow = Window & {
  go?: { app?: { Backend?: PlanBackend } }
  runtime?: WailsRuntime
}

export class PlanUnavailableError extends Error {
  constructor() {
    super('plan_unavailable')
    this.name = 'PlanUnavailableError'
  }
}

function backend(): PlanBackend | null {
  return (window as PlanWindow).go?.app?.Backend ?? null
}

function devPreview(): boolean {
  return import.meta.env.DEV && canUseDevelopmentTestData() && typeof backend()?.GetPlanDay !== 'function'
}

// ---- Browser-preview stand-in (development only) -------------------------

const devBlocks: PlanBlockDTO[] = []
const devListeners = new Set<(day: string | null) => void>()
let devNextId = 1

function devClockMinutes(clock: string): number {
  const [hour, minute] = clock.split(':').map(Number)
  const value = hour * 60 + minute
  // Logical day: 00:00–03:59 belong after 23:59, and 04:00 as an end is the day's end.
  return hour < 4 ? value + 24 * 60 : value
}

function devDayStart(day: string): number {
  return Math.floor(new Date(`${day}T00:00:00`).getTime() / 1000)
}

function devSave(input: PlanBlockInputDTO): number {
  const id = input.id > 0 ? input.id : devNextId++
  const existing = devBlocks.find((block) => block.id === id)
  const endMinutes = input.end === '04:00' ? 28 * 60 : devClockMinutes(input.end)
  const block: PlanBlockDTO = {
    id,
    day: input.day,
    start: input.start,
    end: input.end,
    // Preview only: anchor to local midnight of the day; production instants come from Go.
    startTs: devDayStart(input.day) + devClockMinutes(input.start) * 60,
    endTs: devDayStart(input.day) + endMinutes * 60,
    title: input.title.trim(),
    notes: input.notes?.trim() ? input.notes.trim() : null,
    categoryId: input.categoryId,
    categoryName: '',
    colorHex: '',
    status: existing?.status ?? 'planned',
    completedAtTs: existing?.completedAtTs ?? null,
    remind: input.remind,
    matchedMinutes: 0,
    distractionMinutes: 0,
  }
  if (existing) devBlocks.splice(devBlocks.indexOf(existing), 1, block)
  else devBlocks.push(block)
  devListeners.forEach((listener) => listener(input.day))
  return id
}

// ---- Bindings ---------------------------------------------------------------

export function hasPlanBinding(): boolean {
  return typeof backend()?.GetPlanDay === 'function' || devPreview()
}

export async function getPlanDay(day: string): Promise<PlanDayDTO> {
  if (devPreview()) {
    const blocks = devBlocks.filter((block) => block.day === day).sort((a, b) => a.startTs - b.startTs || a.id - b.id)
    return { day, blocks: blocks.map((block) => ({ ...block })) }
  }
  const method = backend()?.GetPlanDay
  if (typeof method !== 'function') throw new PlanUnavailableError()
  return normalizePlanDay(await method(day))
}

export async function savePlanBlock(input: PlanBlockInputDTO): Promise<number> {
  if (devPreview()) return devSave(input)
  const method = backend()?.SavePlanBlock
  if (typeof method !== 'function') throw new PlanUnavailableError()
  return method(input)
}

export async function setPlanBlockStatus(id: number, status: PlanBlockStatus): Promise<void> {
  if (devPreview()) {
    const block = devBlocks.find((entry) => entry.id === id)
    if (block) {
      block.status = status
      block.completedAtTs = status === 'done' ? Math.floor(Date.now() / 1000) : null
      devListeners.forEach((listener) => listener(block.day))
    }
    return
  }
  const method = backend()?.SetPlanBlockStatus
  if (typeof method !== 'function') throw new PlanUnavailableError()
  return method(id, status)
}

export async function deletePlanBlock(id: number): Promise<void> {
  if (devPreview()) {
    const index = devBlocks.findIndex((entry) => entry.id === id)
    if (index >= 0) {
      const [block] = devBlocks.splice(index, 1)
      devListeners.forEach((listener) => listener(block.day))
    }
    return
  }
  const method = backend()?.DeletePlanBlock
  if (typeof method !== 'function') throw new PlanUnavailableError()
  return method(id)
}

/** plan:updated is invalidation-only ({day}); a missing runtime is a no-op. */
export function onPlanUpdated(callback: (day: string | null) => void): () => void {
  if (devPreview()) {
    devListeners.add(callback)
    return () => devListeners.delete(callback)
  }
  const method = (window as PlanWindow).runtime?.EventsOnMultiple
  if (typeof method !== 'function') return () => undefined
  return method('plan:updated', (raw: unknown) => {
    const day = typeof raw === 'object' && raw !== null ? (raw as Record<string, unknown>).day : null
    callback(typeof day === 'string' ? day : null)
  }, -1)
}
