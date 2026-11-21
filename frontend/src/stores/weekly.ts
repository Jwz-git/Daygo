import { defineStore } from 'pinia'
import { computed, ref, shallowRef, watch } from 'vue'

import { useUIVisibilityStore } from '@/stores/uiVisibility'
import type { DayContextDTO, WeeklyDashboardDTO } from '@/api/dto'
import { getWeeklyDevelopmentFixture } from '@/api/developmentFixtures'
import { getDailyContext, DailyUnavailableError } from '@/api/daily'
import { onTimelineUpdated } from '@/api/timeline'
import { getWeeklyDashboard, hasWeeklyBinding, WeeklyUnavailableError } from '@/api/weekly'
import { shiftWeekStart } from '@/lib/calendarDate'
import {
  buildContextCharts,
  buildHeatmap,
  buildSankey,
  buildTreemap,
  buildWorkflow,
  weeklyChartFacts,
} from '@/stores/weeklyCharts'
import { buildWeeklyDistribution } from '@/stores/weeklyDistribution'

export type WeeklyState = 'loading' | 'unavailable' | 'failure' | 'empty' | 'populated'

export const useWeeklyStore = defineStore('weekly', () => {
  const visibility = useUIVisibilityStore()
  const dashboard = shallowRef<WeeklyDashboardDTO | null>(null)
  // The week before, for the treemap's per-app change only; null when it is
  // unavailable, which simply hides the change badges.
  const previousDashboard = shallowRef<WeeklyDashboardDTO | null>(null)
  const loading = ref(true)
  const unavailable = ref(false)
  const error = ref<unknown>(null)
  const usingDevelopmentFixture = ref(false)
  const currentWeekStart = ref<string | null>(null)
  let requestVersion = 0
  let currentRefreshVersion = 0
  let previousRefreshVersion = 0
  let pendingChange: string | null | undefined
  let stopEvents: (() => void) | null = null

  const state = computed<WeeklyState>(() => {
    if (loading.value) return 'loading'
    if (unavailable.value) return 'unavailable'
    if (error.value !== null) return 'failure'
    if (dashboard.value === null || dashboard.value.trackedMinutes <= 0) return 'empty'
    return 'populated'
  })

  const presentation = computed(() =>
    dashboard.value === null ? null : buildWeeklyDistribution(dashboard.value),
  )

  const facts = computed(() => dashboard.value === null ? null : weeklyChartFacts(dashboard.value))
  const currentCharts = computed(() => {
    const current = facts.value
    if (current === null) return null
    return {
      workflow: buildWorkflow(current),
      heatmap: buildHeatmap(current),
      context: buildContextCharts(current),
      sankey: buildSankey(current),
    }
  })
  const charts = computed(() => {
    const current = facts.value
    const base = currentCharts.value
    if (current === null || base === null) return null
    const previous = previousDashboard.value === null ? [] : weeklyChartFacts(previousDashboard.value)
    return { ...base, treemap: buildTreemap(current, previous) }
  })
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
      void loadPreviousWeek(nextDashboard.weekStart, version)
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
          previousDashboard.value = null
          currentWeekStart.value = fixture.dashboard.weekStart
          usingDevelopmentFixture.value = true
        }
      } else {
        error.value = cause
      }
    } finally {
      if (version === requestVersion) {
        loading.value = false
        flushPendingChange()
      }
    }
  }

  // Best effort: a failure only removes the treemap's change badges, it is
  // never surfaced as a page error.
  async function loadPreviousWeek(weekStart: string, version: number, keep = false): Promise<void> {
    const refresh = ++previousRefreshVersion
    if (!keep) previousDashboard.value = null
    const target = shiftWeekStart(weekStart, -1)
    if (target === null) return
    try {
      const previous = await getWeeklyDashboard(target)
      if (version === requestVersion && refresh === previousRefreshVersion) previousDashboard.value = previous
    } catch {
      if (!keep && version === requestVersion && refresh === previousRefreshVersion) previousDashboard.value = null
    }
  }

  function navigate(weekOffset: -1 | 1): void {
    if (!navigationAvailable.value || dashboard.value === null) return
    if (weekOffset === 1 && !canNavigateForward.value) return
    const target = shiftWeekStart(dashboard.value.weekStart, weekOffset)
    if (target !== null) void load(target)
  }

  async function refreshCurrentWeek(weekStart: string): Promise<void> {
    const version = requestVersion
    const refresh = ++currentRefreshVersion
    try {
      const next = await getWeeklyDashboard(weekStart)
      if (version === requestVersion && refresh === currentRefreshVersion) dashboard.value = next
    } catch {
      // An invalidation does not blank the existing report.
    }
  }

  function flushPendingChange(): void {
    if (!visibility.visible || loading.value || pendingChange === undefined) return
    const changed = pendingChange
    pendingChange = undefined
    const start = dashboard.value?.weekStart
    if (start === undefined) {
      void load()
      return
    }
    const end = shiftWeekStart(start, 1)
    const previous = shiftWeekStart(start, -1)
    if (changed === null || (end !== null && changed >= start && changed < end)) {
      void refreshCurrentWeek(start)
    }
    if (changed === null || (previous !== null && changed >= previous && changed < start)) {
      void loadPreviousWeek(start, requestVersion, true)
    }
  }

  function startEvents(): void {
    if (stopEvents !== null) return
    const stopUpdates = onTimelineUpdated((day) => {
      const changed = day || null
      pendingChange = pendingChange === undefined || pendingChange === changed ? changed : null
      flushPendingChange()
    })
    const stopVisibility = watch(() => visibility.visible, flushPendingChange)
    stopEvents = () => { stopUpdates(); stopVisibility() }
  }

  function stopListening(): void {
    stopEvents?.()
    stopEvents = null
    pendingChange = undefined
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
    charts,
    navigationAvailable,
    canNavigateForward,
    load,
    navigate,
    startEvents,
    stopListening,
  }
})
