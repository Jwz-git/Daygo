import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type { DayContextDTO, WeeklyDashboardDTO } from '@/api/dto'
import { getWeeklyDevelopmentFixture } from '@/api/developmentFixtures'
import { getDailyContext, DailyUnavailableError } from '@/api/daily'
import { onTimelineUpdated } from '@/api/timeline'
import { getWeeklyDashboard, hasWeeklyBinding, WeeklyUnavailableError } from '@/api/weekly'
import { shiftWeekStart } from '@/lib/calendarDate'
import { buildWeeklyPresentation } from '@/stores/weeklyPresentation'

export type WeeklyState = 'loading' | 'unavailable' | 'failure' | 'empty' | 'populated'

export const useWeeklyStore = defineStore('weekly', () => {
  const dashboard = ref<WeeklyDashboardDTO | null>(null)
  const loading = ref(true)
  const unavailable = ref(false)
  const error = ref<unknown>(null)
  const usingDevelopmentFixture = ref(false)
  const currentWeekStart = ref<string | null>(null)
  let requestVersion = 0
  let stopEvents: (() => void) | null = null

  const state = computed<WeeklyState>(() => {
    if (loading.value) return 'loading'
    if (unavailable.value) return 'unavailable'
    if (error.value !== null) return 'failure'
    if (dashboard.value === null || dashboard.value.trackedMinutes <= 0) return 'empty'
    return 'populated'
  })

  const presentation = computed(() =>
    dashboard.value === null ? null : buildWeeklyPresentation(dashboard.value),
  )
  const navigationAvailable = computed(
    () => hasWeeklyBinding() && dashboard.value !== null && !loading.value,
  )
  const canNavigateForward = computed(
    () =>
      navigationAvailable.value &&
      currentWeekStart.value !== null &&
      dashboard.value?.weekStart !== currentWeekStart.value,
  )

  // Resolving the current week goes through the backend: the frontend never
  // derives week boundaries itself (docs/05 §5.3.2 rule 2).
  async function currentWeekStartFromBackend(): Promise<string> {
    const context: DayContextDTO = await getDailyContext('')
    return context.weekStart
  }

  async function load(requestedWeekStart = ''): Promise<void> {
    const version = ++requestVersion
    loading.value = true
    unavailable.value = false
    error.value = null
    usingDevelopmentFixture.value = false

    try {
      // In a plain browser there is no bridge at all: both the context call
      // and the dashboard call are unavailable, and the fixture path decides.
      const weekStart = requestedWeekStart === ''
        ? await currentWeekStartFromBackend().catch((cause: unknown) => {
            if (cause instanceof DailyUnavailableError) throw new WeeklyUnavailableError()
            throw cause
          })
        : requestedWeekStart
      const nextDashboard = await getWeeklyDashboard(weekStart)
      if (version !== requestVersion) return
      dashboard.value = nextDashboard
      if (requestedWeekStart === '') currentWeekStart.value = nextDashboard.weekStart
    } catch (cause: unknown) {
      if (version !== requestVersion) return
      if (cause instanceof WeeklyUnavailableError) {
        const fixture = await getWeeklyDevelopmentFixture()
        if (version !== requestVersion) return
        if (fixture === null) {
          unavailable.value = true
          dashboard.value = null
        } else {
          dashboard.value = fixture.dashboard
          currentWeekStart.value = fixture.dashboard.weekStart
          usingDevelopmentFixture.value = true
        }
      } else {
        error.value = cause
      }
    } finally {
      if (version === requestVersion) loading.value = false
    }
  }

  function navigate(weekOffset: -1 | 1): void {
    if (!navigationAvailable.value || dashboard.value === null) return
    if (weekOffset === 1 && !canNavigateForward.value) return
    const target = shiftWeekStart(dashboard.value.weekStart, weekOffset)
    if (target !== null) void load(target)
  }

  function startEvents(): void {
    if (stopEvents !== null) return
    stopEvents = onTimelineUpdated(() => {
      void load(dashboard.value?.weekStart ?? '')
    })
  }

  function stopListening(): void {
    stopEvents?.()
    stopEvents = null
  }

  return {
    dashboard,
    loading,
    unavailable,
    error,
    usingDevelopmentFixture,
    currentWeekStart,
    state,
    presentation,
    navigationAvailable,
    canNavigateForward,
    load,
    navigate,
    startEvents,
    stopListening,
  }
})
