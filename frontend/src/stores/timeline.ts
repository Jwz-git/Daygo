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
  reprocessCard as reprocessCardApi,
  reprocessDay,
  retryBatches,
  stopRetries,
  TimelineUnavailableError,
  updateCardCategory,
  updateCardDetailedSummary,
  updateCardSummary,
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
  | 'stop-retries'
  | 'delete-batches'
  | 'reprocess-day'
  | 'reprocess-card'

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
  const pendingCardID = ref<number | null>(null)
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
      updateSummary: enabled && actionBindings.updateSummary,
      updateDetailedSummary: enabled && actionBindings.updateDetailedSummary,
      deleteCard: enabled && actionBindings.deleteCard,
      retryBatches: enabled && actionBindings.retryBatches,
      stopRetries: enabled && actionBindings.stopRetries,
      reprocessDay: enabled && actionBindings.reprocessDay,
      reprocessCard: enabled && actionBindings.reprocessCard,
      deleteBatches: enabled && actionBindings.deleteBatches,
      clearHistory: enabled && actionBindings.clearHistory,
      // Category management is always available in the UI when there are categories
      manageCategories: true,
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

  /**
   * Pull the timeline for a day. A silent refresh keeps the current day
   * rendered (loading stays false) while re-pulling behind it, so an
   * edit/delete confirmation does not unmount the track and lose scroll
   * position. It degrades to a full load whenever no day is on screen.
   */
  async function load(requestedDay = '', options: { silent?: boolean } = {}): Promise<void> {
    const version = ++requestVersion
    const silent = options.silent === true && day.value !== null && loading.value === false
    if (!silent) {
      loading.value = true
      unavailable.value = false
      error.value = null
      actionError.value = null
      usingDevelopmentFixture.value = false
    }

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
      // A failed silent refresh keeps the stale day on screen; the next
      // explicit load or event re-pull surfaces the error normally.
      if (silent) return
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

  function saveCardEdits(
    cardID: number,
    edits: { title?: string; category?: string; summary?: string; detailedSummary?: string },
  ): Promise<boolean> {
    return runAction('update-card', async () => {
      if (edits.title !== undefined && edits.title.trim() !== '') {
        await updateCardTitle(cardID, edits.title.trim())
      }
      if (edits.category !== undefined && edits.category.trim() !== '') {
        await updateCardCategory(cardID, edits.category.trim())
      }
      if (edits.summary !== undefined) {
        await updateCardSummary(cardID, edits.summary.trim())
      }
      if (edits.detailedSummary !== undefined) {
        await updateCardDetailedSummary(cardID, edits.detailedSummary.trim())
      }
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

  function stopFailureRetries(batchIDs: number[]): Promise<boolean> {
    return runAction('stop-retries', () => stopRetries(batchIDs))
  }

  function dismissFailure(batchIDs: number[]): Promise<boolean> {
    return runAction('delete-batches', () => deleteBatches(batchIDs))
  }

  function reprocessCurrentDay(day: string): Promise<boolean> {
    return runAction('reprocess-day', () => reprocessDay(day))
  }

  function reprocessCard(cardID: number): Promise<boolean> {
    // The rewrite happens inside this call, so the card has to show its
    // regenerating state from here: no batch goes pending, and processingRanges
    // can therefore never report it.
    pendingCardID.value = cardID
    return runAction('reprocess-card', () => reprocessCardApi(cardID)).finally(() => {
      pendingCardID.value = null
    })
  }

  function startEvents(): void {
    if (stopEvents !== null) return
    stopEvents = onTimelineUpdated((updatedDay) => {
      if (updatedDay === null || updatedDay === context.value?.day) {
        void load(context.value?.day ?? '', { silent: true })
      }
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
    pendingCardID,
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
    stopFailureRetries,
    dismissFailure,
    reprocessCurrentDay,
    reprocessCard,
    startEvents,
    stopListening,
  }
})
