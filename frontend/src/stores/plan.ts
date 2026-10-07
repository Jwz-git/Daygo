import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type { PlanBlockDTO, PlanBlockInputDTO, PlanBlockStatus } from '@/api/dto'
import { onGoalUpdated } from '@/api/daily'
import { onTimelineUpdated } from '@/api/timeline'
import {
  deletePlanBlock,
  getPlanDay,
  hasPlanBinding,
  onPlanUpdated,
  savePlanBlock,
  setPlanBlockStatus,
} from '@/api/plan'

/** What went wrong with the last write, for the panel's message. */
export type PlanWriteError = 'invalid' | 'readonly' | 'failed'

/** A request from the track or week grid for the inspector panel to show
 * (and optionally open the editor for) one block; nonce re-fires repeats. */
export interface PlanFocusRequest {
  id: number
  edit: boolean
  nonce: number
}

/*
 * One logical day's plan. Writes are not optimistic: the backend emits
 * plan:updated and the listener re-pulls (docs/05 §5.5.5), so a block marked
 * done by the CLI or an AI shows up the same way as one marked here.
 */
export const usePlanStore = defineStore('plan', () => {
  const day = ref<string | null>(null)
  const blocks = ref<PlanBlockDTO[]>([])
  const loading = ref(false)
  const loadFailed = ref(false)
  const pending = ref(false)
  const writeError = ref<PlanWriteError | null>(null)
  /** The week grid's blocks, by logical day; separate from the day's own list. */
  const week = ref<Record<string, PlanBlockDTO[]>>({})
  const focusRequest = ref<PlanFocusRequest | null>(null)
  let requestVersion = 0
  let weekVersion = 0
  const weekDayVersions = new Map<string, number>()
  let focusNonce = 0
  let stopEvents: (() => void) | null = null

  const available = computed(() => hasPlanBinding())

  async function load(nextDay: string): Promise<void> {
    if (nextDay === '') return
    const version = ++requestVersion
    if (day.value !== nextDay) blocks.value = []
    day.value = nextDay
    loading.value = true
    loadFailed.value = false
    try {
      const result = await getPlanDay(nextDay)
      if (version !== requestVersion) return
      blocks.value = result.blocks
    } catch {
      if (version !== requestVersion) return
      loadFailed.value = true
    } finally {
      if (version === requestVersion) loading.value = false
    }
  }

  /** Loads every day of the week grid; a day that fails just draws no plan. */
  async function loadWeek(days: string[]): Promise<void> {
    const keys = days.filter((key) => key !== '')
    const version = ++weekVersion
    weekDayVersions.clear()
    week.value = Object.fromEntries(keys.map((key) => [key, week.value[key] ?? []]))
    const results = await Promise.allSettled(keys.map((key) => getPlanDay(key)))
    if (version !== weekVersion) return
    const next: Record<string, PlanBlockDTO[]> = {}
    keys.forEach((key, index) => {
      const result = results[index]
      next[key] = result?.status === 'fulfilled' && !weekDayVersions.has(key)
        ? result.value.blocks : (week.value[key] ?? [])
    })
    week.value = next
  }

  async function reloadWeekDay(changed: string): Promise<void> {
    if (!(changed in week.value)) return
    const version = weekVersion
    const dayVersion = (weekDayVersions.get(changed) ?? 0) + 1
    weekDayVersions.set(changed, dayVersion)
    try {
      const result = await getPlanDay(changed)
      if (version !== weekVersion || dayVersion !== weekDayVersions.get(changed)) return
      week.value = { ...week.value, [changed]: result.blocks }
    } catch {
      // Keep the drawn blocks; the next week load retries.
    }
  }

  function requestFocus(id: number, edit: boolean): void {
    focusRequest.value = { id, edit, nonce: ++focusNonce }
  }

  async function write(action: () => Promise<unknown>): Promise<boolean> {
    if (pending.value) return false
    pending.value = true
    writeError.value = null
    try {
      await action()
      return true
    } catch (cause) {
      writeError.value = writeErrorOf(cause)
      return false
    } finally {
      pending.value = false
    }
  }

  const save = (input: PlanBlockInputDTO) => write(() => savePlanBlock(input))
  const setStatus = (id: number, status: PlanBlockStatus) => write(() => setPlanBlockStatus(id, status))
  const remove = (id: number) => write(() => deletePlanBlock(id))

  function startListening(): void {
    if (stopEvents !== null) return
    const reload = (changed: string | null) => {
      if (day.value !== null && (changed === null || changed === day.value)) void load(day.value)
      if (changed === null) void loadWeek(Object.keys(week.value))
      else void reloadWeekDay(changed)
    }
    // Reviews derive from cards and goal categories as well as the plan itself.
    const stops = [onPlanUpdated(reload), onTimelineUpdated(reload), onGoalUpdated(reload)]
    stopEvents = () => stops.forEach((stop) => stop())
  }

  function stopListening(): void {
    stopEvents?.()
    stopEvents = null
  }

  return {
    day, blocks, loading, loadFailed, pending, writeError, available, week, focusRequest,
    load, loadWeek, save, setStatus, remove, requestFocus, startListening, stopListening,
  }
})

/** Binding errors arrive as "daygo:<code>: <message>". */
export function writeErrorOf(cause: unknown): PlanWriteError {
  const text = cause instanceof Error ? cause.message : String(cause)
  const code = /daygo:([a-z_]+):/.exec(text)?.[1]
  if (code === 'invalid_argument') return 'invalid'
  if (code === 'not_capture_owner') return 'readonly'
  return 'failed'
}
