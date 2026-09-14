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
  deleteBatches,
  deleteCard,
  getDayContext,
  getTimelineActionAvailability,
  getTimelineCapabilities,
  getTimelineDay,
  hasTimelineDayBinding,
  onTimelineUpdated,
  retryBatches,
  TimelineUnavailableError,
  updateCardCategory,
  updateCardTitle,
} from '@/api/timeline'

export type TimelineState =
  | 'loading'
  | 'unavailable'
  | 'empty'
  | 'processing'
  | 'failure'
  | 'populated'

export type TimelineAction =
  | 'update-card'
  | 'delete-card'
  | 'retry-batches'
  | 'delete-batches'

export const useTimelineStore = defineStore('timeline', () => {
  const context = ref<DayContextDTO | null>(null)
  const day = ref<TimelineDayDTO | null>(null)
  const capabilities = ref<CapabilitiesDTO | null>(null)
  const loading = ref(true)
  const unavailable = ref(false)
  const error = ref<unknown>(null)
  const selectedCardID = ref<number | null>(null)
  const selectedFailureTs = ref<number | null>(null)
  const categoryFilter = ref<string | null>(null)
  const usingDevelopmentFixture = ref(false)
  const pendingAction = ref<TimelineAction | null>(null)
  const actionError = ref<unknown>(null)
  const actionBindings = getTimelineActionAvailability()
  let requestVersion = 0
  let stopEvents: (() => void) | null = null

  const cards = computed<TimelineCardDTO[]>(() => {
    const entries = day.value?.cards ?? []
    const filter = categoryFilter.value
    return filter === null ? entries : entries.filter((card) => card.category === filter)
  })

  const selectedCard = computed(
    () => day.value?.cards?.find((card) => card.id === selectedCardID.value) ?? null,
  )

  const selectedFailure = computed(
    () => day.value?.failures?.find((failure) => failure.startTs === selectedFailureTs.value) ?? null,
  )

  const actionAvailability = computed(() => {
    const enabled = capabilities.value?.features?.includes('timeline') ?? false
    return {
      updateCategory: enabled && actionBindings.updateCategory,
      updateTitle: enabled && actionBindings.updateTitle,
      deleteCard: enabled && actionBindings.deleteCard,
      retryBatches: enabled && actionBindings.retryBatches,
      deleteBatches: enabled && actionBindings.deleteBatches,
      clearHistory: enabled && actionBindings.clearHistory,
    }
  })

  const state = computed<TimelineState>(() => {
    if (loading.value) return 'loading'
    if (unavailable.value) return 'unavailable'
    if (error.value !== null && (day.value?.cards?.length ?? 0) === 0) return 'failure'
    if ((day.value?.cards?.length ?? 0) > 0) return 'populated'
    if ((day.value?.processingRanges?.length ?? 0) > 0) return 'processing'
    if ((day.value?.failures?.length ?? 0) > 0) return 'failure'
    return 'empty'
  })

  async function load(requestedDay = ''): Promise<void> {
    const version = ++requestVersion
    loading.value = true
    unavailable.value = false
    error.value = null
    actionError.value = null
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
      if (selectedFailureTs.value !== null && selectedFailure.value === null) {
        selectedFailureTs.value = null
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
    selectedFailureTs.value = null
    actionError.value = null
  }

  function selectFailure(startTs: number | null): void {
    selectedFailureTs.value = startTs
    if (startTs !== null) selectedCardID.value = null
    actionError.value = null
  }

  function setCategoryFilter(category: string | null): void {
    categoryFilter.value = category
    if (selectedCard.value !== null && category !== null && selectedCard.value.category !== category) {
      selectedCardID.value = null
    }
  }

  async function runAction(action: TimelineAction, operation: () => Promise<void>): Promise<boolean> {
    if (pendingAction.value !== null) return false
    pendingAction.value = action
    actionError.value = null
    try {
      await operation()
      return true
    } catch (cause: unknown) {
      actionError.value = cause
      return false
    } finally {
      pendingAction.value = null
    }
  }

  function saveCardEdits(cardID: number, title: string, category: string): Promise<boolean> {
    const nextTitle = title.trim()
    const nextCategory = category.trim()
    return runAction('update-card', async () => {
      if (nextTitle !== '') await updateCardTitle(cardID, nextTitle)
      if (nextCategory !== '') await updateCardCategory(cardID, nextCategory)
    })
  }

  async function removeCard(cardID: number): Promise<boolean> {
    const succeeded = await runAction('delete-card', () => deleteCard(cardID))
    if (succeeded) selectedCardID.value = null
    return succeeded
  }

  function retryFailure(batchIDs: number[]): Promise<boolean> {
    return runAction('retry-batches', () => retryBatches(batchIDs))
  }

  function dismissFailure(batchIDs: number[]): Promise<boolean> {
    return runAction('delete-batches', () => deleteBatches(batchIDs))
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
    pendingAction,
    actionError,
    actionAvailability,
    cards,
    selectedCard,
    selectedFailure,
    selectedFailureTs,
    state,
    dayNavigationAvailable: computed(() => hasTimelineDayBinding()),
    load,
    selectCard,
    selectFailure,
    setCategoryFilter,
    saveCardEdits,
    removeCard,
    retryFailure,
    dismissFailure,
    startEvents,
    stopListening,
  }
})
