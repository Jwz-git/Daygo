import { defineStore } from 'pinia'
import { ref } from 'vue'

import { getDayGoal, hasGoalBinding, onGoalUpdated, saveDayGoal } from '@/api/daily'
import type { DayGoalDTO } from '@/api/dto'
import { deferHiddenRefresh } from '@/lib/hiddenRefresh'
import { useUIVisibilityStore } from '@/stores/uiVisibility'

export const useDayGoalStore = defineStore('dayGoal', () => {
  const visibility = useUIVisibilityStore()
  const goal = ref<DayGoalDTO | null>(null)
  const unavailable = ref(false)
  const error = ref<unknown>(null)
  const saving = ref(false)
  let day: string | null = null
  let version = 0
  let stop: (() => void) | null = null

  async function load(nextDay: string | null): Promise<void> {
    const request = ++version
    if (day !== nextDay) goal.value = null
    day = nextDay
    error.value = null
    unavailable.value = !hasGoalBinding()
    if (nextDay === null || unavailable.value) {
      goal.value = null
      return
    }
    try {
      const next = await getDayGoal(nextDay)
      if (request === version) goal.value = next
    } catch (cause: unknown) {
      if (request !== version) return
      goal.value = null
      error.value = cause
    }
  }

  async function save(next: DayGoalDTO): Promise<void> {
    const selectedDay = day
    const request = version
    if (selectedDay === null || saving.value) return
    saving.value = true
    error.value = null
    try {
      await saveDayGoal({ ...next, day: selectedDay })
      // Re-read after a confirmed write when no event has already started it.
      if (request === version) await load(selectedDay)
    } catch (cause: unknown) {
      if (day === selectedDay) error.value = cause
    } finally {
      saving.value = false
    }
  }

  function startListening(): void {
    if (stop !== null) return
    const refresh = deferHiddenRefresh<true>(() => visibility.visible, () => {
      if (day !== null) void load(day)
    })
    const stopUpdates = onGoalUpdated((changed) => {
      if (day !== null && (!changed || changed === day)) refresh.request(true)
    })
    stop = () => {
      stopUpdates()
      refresh.stop()
    }
  }

  function stopListening(): void {
    stop?.()
    stop = null
    day = null
    goal.value = null
    version++
  }

  return { goal, unavailable, error, saving, load, save, startListening, stopListening }
})
