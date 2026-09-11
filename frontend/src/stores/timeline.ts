import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  CapabilitiesDTO,
  DayContextDTO,
  TimelineCardDTO,
  TimelineDayDTO,
} from '@/api/dto'
import { getTimelineDevelopmentFixture } from '@/api/developmentFixtures'
import {
  getDayContext,
  getTimelineCapabilities,
  getTimelineDay,
  hasDayContextBinding,
  onTimelineUpdated,
  TimelineUnavailableError,
} from '@/api/timeline'

export type TimelineState =
  | 'loading'
  | 'unavailable'
  | 'empty'
  | 'processing'
  | 'failure'
  | 'populated'

export const useTimelineStore = defineStore('timeline', () => {
  const context = ref<DayContextDTO | null>(null)
  const day = ref<TimelineDayDTO | null>(null)
  const capabilities = ref<CapabilitiesDTO | null>(null)
  const loading = ref(true)
  const unavailable = ref(false)
  const error = ref<unknown>(null)
  const selectedCardID = ref<number | null>(null)
  const categoryFilter = ref<string | null>(null)
  const usingDevelopmentFixture = ref(false)
  let requestVersion = 0
  let stopEvents: (() => void) | null = null

  const cards = computed<TimelineCardDTO[]>(() => {
    const entries = day.value?.cards ?? []
    const filter = categoryFilter.value
    return filter === null ? entries : entries.filter((card) => card.category === filter)
  })

  const selectedCard = computed(
    () => day.value?.cards.find((card) => card.id === selectedCardID.value) ?? null,
  )

  const state = computed<TimelineState>(() => {
    if (loading.value) return 'loading'
    if (unavailable.value) return 'unavailable'
    if (error.value !== null && (day.value?.cards.length ?? 0) === 0) return 'failure'
    if ((day.value?.cards.length ?? 0) > 0) return 'populated'
    if ((day.value?.processingRanges.length ?? 0) > 0) return 'processing'
    if ((day.value?.failures.length ?? 0) > 0) return 'failure'
    return 'empty'
  })

  async function load(requestedDay = ''): Promise<void> {
    const version = ++requestVersion
    loading.value = true
    unavailable.value = false
    error.value = null
    usingDevelopmentFixture.value = false

    try {
      const nextContext = await getDayContext(requestedDay)
      if (version !== requestVersion) return
      context.value = nextContext

      const [nextDay, nextCapabilities] = await Promise.all([
        getTimelineDay(nextContext.day),
        getTimelineCapabilities(),
      ])
      if (version !== requestVersion) return

      day.value = nextDay
      capabilities.value = nextCapabilities
      if (selectedCardID.value !== null && selectedCard.value === null) {
        selectedCardID.value = null
      }
    } catch (cause: unknown) {
      if (version !== requestVersion) return
      if (cause instanceof TimelineUnavailableError) {
        const fixture = await getTimelineDevelopmentFixture()
        if (version !== requestVersion) return
        if (fixture === null) {
          unavailable.value = true
          day.value = null
        } else {
          context.value = fixture.context
          day.value = fixture.day
          capabilities.value = fixture.capabilities
          usingDevelopmentFixture.value = true
        }
      } else {
        error.value = cause
      }
    } finally {
      if (version === requestVersion) loading.value = false
    }
  }

  function selectCard(id: number | null): void {
    selectedCardID.value = id
  }

  function setCategoryFilter(category: string | null): void {
    categoryFilter.value = category
    if (selectedCard.value !== null && category !== null && selectedCard.value.category !== category) {
      selectedCardID.value = null
    }
  }

  function startEvents(): void {
    if (stopEvents !== null) return
    stopEvents = onTimelineUpdated((updatedDay) => {
      if (updatedDay === null || updatedDay === context.value?.day) void load(context.value?.day ?? '')
    })
  }

  function stopListening(): void {
    stopEvents?.()
    stopEvents = null
  }

  return {
    context,
    day,
    capabilities,
    loading,
    error,
    selectedCardID,
    categoryFilter,
    usingDevelopmentFixture,
    cards,
    selectedCard,
    state,
    dayNavigationAvailable: computed(() => hasDayContextBinding()),
    load,
    selectCard,
    setCategoryFilter,
    startEvents,
    stopListening,
  }
})
