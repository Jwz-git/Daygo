import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  CapabilitiesDTO,
  CategoryDTO,
  DailyRecapDTO,
  DayContextDTO,
  DayGoalDTO,
  JournalDayDTO,
  TimelineCardDTO,
  TimelineDayDTO,
} from '@/api/dto'
import { getDailyDevelopmentFixture } from '@/api/developmentFixtures'
import {
  DailyUnavailableError,
  generateDailyRecap,
  getDailyCapabilities,
  getDailyContext,
  getDailyRecap,
  getDailyTimeline,
  getDayGoal,
  getJournalDay,
  hasDailyDayBinding,
  hasDailyRecapBinding,
  hasGoalBinding,
  hasJournalBinding,
  hasRecapGenerationBinding,
  onGoalUpdated,
  onJournalUpdated,
  onRecapUpdated,
  saveDayGoal,
  saveJournalDay,
} from '@/api/daily'
import { onTimelineUpdated } from '@/api/timeline'
import { categoryKey } from '@/lib/categoryLabel'

export const SLOT_SECONDS = 15 * 60
const MINIMUM_WINDOW_SLOTS = 36

export interface DailyWorkflowCell {
  occupancy: number
  title: string | null
  hasDistraction: boolean
}

export interface DailyWorkflowRow {
  id: string
  name: string
  colorHex: string
  minutes: number
  cells: DailyWorkflowCell[]
}

export interface DailyWorkflowTick {
  timestamp: number
  position: number
}

export interface DailyMetrics {
  contextSwitches: number
  interruptions: number
  focusedMinutes: number
  distractedMinutes: number
  transitioningMinutes: number
}

export interface DailyWorkflowDistractionMarker {
  id: string
  title: string
  startTs: number
  endTs: number
  durationMinutes: number
}

export interface DailyPresentation {
  rows: DailyWorkflowRow[]
  ticks: DailyWorkflowTick[]
  metrics: DailyMetrics
  distractionMarkers: DailyWorkflowDistractionMarker[]
  hasDistractionCategory: boolean
  windowStartTs: number
  windowEndTs: number
  slotCount: number
}

export type DailyState = 'loading' | 'unavailable' | 'failure' | 'empty' | 'populated'

export function isDistractionCategoryKey(key: string): boolean {
  const normalized = key.trim().toLowerCase()
  return normalized === 'distraction' || normalized === 'distractions'
}

export function parseClockToMinutes(clock: string): number | null {
  const match = clock.trim().match(/^(\d{1,2}):(\d{2})(?:\s*([ap]m))?$/i)
  if (!match) return null
  let hours = parseInt(match[1]!, 10)
  const minutes = parseInt(match[2]!, 10)
  const meridian = match[3]?.toLowerCase()
  if (meridian === 'pm' && hours < 12) hours += 12
  else if (meridian === 'am' && hours === 12) hours = 0
  return hours * 60 + minutes
}

function safeColor(value: string): string {
  return /^#[0-9a-f]{6}$/i.test(value) ? value : '#7D7A84'
}

function validCards(day: TimelineDayDTO): TimelineCardDTO[] {
  return day.cards
    .filter((card) => card.endTs > card.startTs && card.endTs - card.startTs <= 4 * 3600)
    .filter((card) => card.endTs > day.dayStartTs && card.startTs < day.dayEndTs)
    .sort((left, right) => left.startTs - right.startTs || left.endTs - right.endTs)
}

function resolveWindow(day: TimelineDayDTO, cards: TimelineCardDTO[]) {
  const totalSlots = Math.max(1, Math.ceil((day.dayEndTs - day.dayStartTs) / SLOT_SECONDS))
  if (cards.length === 0) {
    const startSlot = Math.min(20, Math.max(0, totalSlots - MINIMUM_WINDOW_SLOTS))
    return {
      startSlot,
      endSlot: Math.min(totalSlots, startSlot + MINIMUM_WINDOW_SLOTS),
    }
  }

  const firstTs = Math.max(day.dayStartTs, cards[0]?.startTs ?? day.dayStartTs)
  const lastTs = Math.min(
    day.dayEndTs,
    cards.reduce((latest, card) => Math.max(latest, card.endTs), firstTs),
  )
  let startSlot = Math.max(0, Math.floor((firstTs - day.dayStartTs) / SLOT_SECONDS / 4) * 4)
  let endSlot = Math.min(
    totalSlots,
    Math.ceil((lastTs - day.dayStartTs) / SLOT_SECONDS / 4) * 4,
  )

  if (endSlot - startSlot < MINIMUM_WINDOW_SLOTS) {
    endSlot = Math.min(totalSlots, startSlot + MINIMUM_WINDOW_SLOTS)
    startSlot = Math.max(0, endSlot - MINIMUM_WINDOW_SLOTS)
  }

  return { startSlot, endSlot: Math.max(startSlot + 1, endSlot) }
}

function categoryRows(day: TimelineDayDTO, cards: TimelineCardDTO[]): CategoryDTO[] {
  const known = day.categories
    .filter((category) => !category.isSystem)
    .sort((left, right) => left.sortOrder - right.sortOrder)
  const knownKeys = new Set(known.map((category) => categoryKey(category.name)))
  const extras = new Map<string, CategoryDTO>()

  for (const card of cards) {
    const key = categoryKey(card.category)
    if (knownKeys.has(key) || extras.has(key)) continue
    extras.set(key, {
      id: `unknown-${key}`,
      name: card.category.trim() || 'Uncategorized',
      colorHex: '#7D7A84',
      details: '',
      sortOrder: known.length + extras.size,
      isSystem: false,
      isIdle: card.isIdle,
      createdAtTs: 0,
      updatedAtTs: 0,
    })
  }

  return [...known, ...extras.values()]
}

export function buildDailyPresentation(day: TimelineDayDTO): DailyPresentation {
  const cards = validCards(day)
  const { startSlot, endSlot } = resolveWindow(day, cards)
  const slotCount = endSlot - startSlot
  const windowStartTs = day.dayStartTs + startSlot * SLOT_SECONDS
  const windowEndTs = day.dayStartTs + endSlot * SLOT_SECONDS
  const categories = categoryRows(day, cards)
  const categoryByKey = new Map(categories.map((category) => [categoryKey(category.name), category]))
  const rows = categories.map((category): DailyWorkflowRow => {
    const matchingCards = cards.filter(
      (card) => categoryKey(card.category) === categoryKey(category.name),
    )
    const cells = Array.from({ length: slotCount }, (_, index): DailyWorkflowCell => {
      const slotStart = windowStartTs + index * SLOT_SECONDS
      const slotEnd = slotStart + SLOT_SECONDS
      let occupiedSeconds = 0
      let strongestOverlap = 0
      let title: string | null = null
      let hasDistraction = false

      for (const card of matchingCards) {
        const overlap = Math.max(0, Math.min(card.endTs, slotEnd) - Math.max(card.startTs, slotStart))
        if (overlap === 0) continue
        occupiedSeconds += overlap
        hasDistraction ||= card.distractions.length > 0
        if (overlap > strongestOverlap) {
          strongestOverlap = overlap
          title = card.title
        }
      }

      return {
        occupancy: Math.min(1, occupiedSeconds / SLOT_SECONDS),
        title,
        hasDistraction,
      }
    })
    const minutes = matchingCards.reduce(
      (total, card) =>
        total +
        Math.max(
          0,
          Math.min(day.dayEndTs, card.endTs) - Math.max(day.dayStartTs, card.startTs),
        ) /
          60,
      0,
    )

    return {
      id: category.id,
      name: category.name,
      colorHex: safeColor(category.colorHex),
      minutes: Math.round(minutes),
      cells,
    }
  })

  const hasDistractionCategory = categories.some((category) =>
    isDistractionCategoryKey(category.name),
  )
  const rawMarkers: DailyWorkflowDistractionMarker[] = []

  // Embedded micro-distractions are valid even when the user has no
  // top-level Distraction category. Build the dedicated track from markers
  // independently; macro Distraction cards are included when that category
  // exists.
  for (const card of cards) {
      // Source 1: Full cards categorized as "Distraction"
      if (isDistractionCategoryKey(card.category)) {
        const clippedStart = Math.max(card.startTs, windowStartTs)
        const clippedEnd = Math.min(card.endTs, windowEndTs)
        if (clippedEnd > clippedStart) {
          rawMarkers.push({
            id: `distraction-macro-${rawMarkers.length}`,
            title: card.title,
            startTs: clippedStart,
            endTs: clippedEnd,
            durationMinutes: Math.max(1, Math.round((clippedEnd - clippedStart) / 60)),
          })
        }
      }

      // Source 2: Mini distractions embedded within any card
      if (card.distractions && card.distractions.length > 0) {
        for (const distraction of card.distractions) {
          let dStartTs = card.startTs
          let dEndTs = card.endTs

          const parentStartMin = parseClockToMinutes(card.start)
          const rawStartMin = parseClockToMinutes(distraction.startTime)
          const rawEndMin = parseClockToMinutes(distraction.endTime)

          if (parentStartMin !== null && rawStartMin !== null && rawEndMin !== null) {
            let startOffset = (rawStartMin - parentStartMin) * 60
            if (startOffset < 0) startOffset += 24 * 3600
            let endOffset = (rawEndMin - parentStartMin) * 60
            if (endOffset < startOffset) endOffset += 24 * 3600

            dStartTs = Math.min(card.endTs, Math.max(card.startTs, card.startTs + startOffset))
            dEndTs = Math.min(card.endTs, Math.max(dStartTs + 60, card.startTs + endOffset))
          }

          const clippedStart = Math.max(dStartTs, windowStartTs)
          const clippedEnd = Math.min(dEndTs, windowEndTs)
          if (clippedEnd > clippedStart) {
            rawMarkers.push({
              id: `distraction-mini-${rawMarkers.length}`,
              title: distraction.title,
              startTs: clippedStart,
              endTs: clippedEnd,
              durationMinutes: Math.max(1, Math.round((clippedEnd - clippedStart) / 60)),
            })
          }
        }
      }
  }

  // Merge overlapping or adjacent markers (within 2 minutes)
  const distractionMarkers: DailyWorkflowDistractionMarker[] = []
  if (rawMarkers.length > 0) {
    rawMarkers.sort((a, b) => a.startTs - b.startTs)
    let currentStart = rawMarkers[0]!.startTs
    let currentEnd = rawMarkers[0]!.endTs
    let currentTitles = [rawMarkers[0]!.title]

    for (let i = 1; i < rawMarkers.length; i++) {
      const marker = rawMarkers[i]!
      if (marker.startTs <= currentEnd + 120) {
        currentEnd = Math.max(currentEnd, marker.endTs)
        if (!currentTitles.includes(marker.title)) {
          currentTitles.push(marker.title)
        }
      } else {
        distractionMarkers.push({
          id: `distraction-merged-${distractionMarkers.length}`,
          title: currentTitles.filter(Boolean).join(', '),
          startTs: currentStart,
          endTs: currentEnd,
          durationMinutes: Math.max(1, Math.round((currentEnd - currentStart) / 60)),
        })
        currentStart = marker.startTs
        currentEnd = marker.endTs
        currentTitles = [marker.title]
      }
    }
    distractionMarkers.push({
      id: `distraction-merged-${distractionMarkers.length}`,
      title: currentTitles.filter(Boolean).join(', '),
      startTs: currentStart,
      endTs: currentEnd,
      durationMinutes: Math.max(1, Math.round((currentEnd - currentStart) / 60)),
    })
  }

  let contextSwitches = 0
  let interruptions = 0
  let focusedMinutes = 0
  let distractedMinutes = 0
  let transitioningMinutes = 0
  let previousCategory: string | null = null
  let previousEndTs: number | null = null

  for (const card of cards) {
    const category = categoryByKey.get(categoryKey(card.category))
    const durationMinutes =
      Math.max(
        0,
        Math.min(day.dayEndTs, card.endTs) - Math.max(day.dayStartTs, card.startTs),
      ) / 60
    const isDistraction = isDistractionCategoryKey(card.category)
    const isIdle = category?.isIdle ?? card.isIdle

    if (isIdle || isDistraction) distractedMinutes += durationMinutes
    else focusedMinutes += durationMinutes
    if (card.distractions.length > 0) interruptions += 1

    const currentCategory = categoryKey(card.category)
    if (previousCategory !== null && previousCategory !== currentCategory) contextSwitches += 1
    previousCategory = currentCategory

    if (previousEndTs !== null && card.startTs > previousEndTs) {
      transitioningMinutes += (card.startTs - previousEndTs) / 60
    }
    previousEndTs = Math.max(previousEndTs ?? card.endTs, card.endTs)
  }

  const ticks = Array.from(
    { length: Math.floor(slotCount / 4) + 1 },
    (_, index): DailyWorkflowTick => {
      const tickSlot = Math.min(slotCount, index * 4)
      return {
        timestamp: windowStartTs + tickSlot * SLOT_SECONDS,
        position: tickSlot / slotCount,
      }
    },
  )

  if (ticks.at(-1)?.position !== 1) {
    ticks.push({ timestamp: windowEndTs, position: 1 })
  }

  return {
    rows,
    ticks,
    metrics: {
      contextSwitches,
      interruptions,
      focusedMinutes: Math.round(focusedMinutes),
      distractedMinutes: Math.round(distractedMinutes),
      transitioningMinutes: Math.round(transitioningMinutes),
    },
    distractionMarkers,
    hasDistractionCategory,
    windowStartTs,
    windowEndTs,
    slotCount,
  }
}

export const useDailyStore = defineStore('daily', () => {
  const context = ref<DayContextDTO | null>(null)
  const day = ref<TimelineDayDTO | null>(null)
  const recap = ref<DailyRecapDTO | null>(null)
  const capabilities = ref<CapabilitiesDTO | null>(null)
  const loading = ref(true)
  const unavailable = ref(false)
  const error = ref<unknown>(null)
  const recapUnavailable = ref(false)
  const recapError = ref<unknown>(null)
  const recapGenerating = ref(false)
  const recapGenerateError = ref<unknown>(null)
  const usingDevelopmentFixture = ref(false)
  const journal = ref<JournalDayDTO | null>(null)
  const journalUnavailable = ref(false)
  const journalError = ref<unknown>(null)
  const journalSaving = ref(false)
  const goal = ref<DayGoalDTO | null>(null)
  const goalUnavailable = ref(false)
  const goalError = ref<unknown>(null)
  const goalSaving = ref(false)
  let requestVersion = 0
  let stopEvents: (() => void) | null = null
  let stopJournalEvents: (() => void) | null = null
  let stopGoalEvents: (() => void) | null = null
  let stopRecapEvents: (() => void) | null = null

  const state = computed<DailyState>(() => {
    if (loading.value) return 'loading'
    if (unavailable.value) return 'unavailable'
    if (error.value !== null) return 'failure'
    if ((day.value?.cards.length ?? 0) === 0 && recap.value === null) return 'empty'
    return 'populated'
  })

  const presentation = computed(() => (day.value ? buildDailyPresentation(day.value) : null))

  async function load(requestedDay = ''): Promise<void> {
    const version = ++requestVersion
    loading.value = true
    unavailable.value = false
    error.value = null
    recapUnavailable.value = false
    recapError.value = null
    recapGenerateError.value = null
    journalUnavailable.value = false
    journalError.value = null
    goalUnavailable.value = false
    goalError.value = null
    usingDevelopmentFixture.value = false

    try {
      const nextContext = await getDailyContext(requestedDay)
      if (version !== requestVersion) return
      const [nextDay, nextCapabilities] = await Promise.all([
        getDailyTimeline(nextContext.day),
        getDailyCapabilities(),
      ])
      if (version !== requestVersion) return

      context.value = nextContext
      day.value = nextDay
      capabilities.value = nextCapabilities

      if (!hasDailyRecapBinding()) {
        recap.value = null
        recapUnavailable.value = true
      } else {
        try {
          const nextRecap = await getDailyRecap(nextContext.standupDay)
          if (version !== requestVersion) return
          recap.value = nextRecap
        } catch (cause: unknown) {
          if (version !== requestVersion) return
          recap.value = null
          recapError.value = cause
        }
      }
      await loadJournalAndGoal(nextContext.day, version)
    } catch (cause: unknown) {
      if (version !== requestVersion) return
      if (cause instanceof DailyUnavailableError) {
        const fixture = await getDailyDevelopmentFixture()
        if (version !== requestVersion) return
        if (fixture === null) {
          unavailable.value = true
          context.value = null
          day.value = null
          recap.value = null
          capabilities.value = null
        } else {
          context.value = fixture.context
          day.value = fixture.timeline
          recap.value = fixture.recap
          capabilities.value = fixture.capabilities
          usingDevelopmentFixture.value = true
        }
        // Journal/goal bindings are absent in a plain browser too; this marks
        // their panels unavailable instead of leaving stale values behind.
        journal.value = null
        journalUnavailable.value = true
        goal.value = null
        goalUnavailable.value = true
      } else {
        error.value = cause
      }
    } finally {
      if (version === requestVersion) loading.value = false
    }
  }

  // Journal and goal panels load per-day, independently of the timeline day:
  // goals are often set before any activity exists. Unavailable bindings
  // degrade to their panel's unavailable state, not a page failure.
  async function loadJournalAndGoal(day: string, version: number): Promise<void> {
    if (!hasJournalBinding()) {
      journal.value = null
      journalUnavailable.value = true
    } else {
      try {
        const next = await getJournalDay(day)
        if (version !== requestVersion) return
        journal.value = next
      } catch (cause: unknown) {
        if (version !== requestVersion) return
        journal.value = null
        journalError.value = cause
      }
    }
    if (!hasGoalBinding()) {
      goal.value = null
      goalUnavailable.value = true
    } else {
      try {
        const next = await getDayGoal(day)
        if (version !== requestVersion) return
        goal.value = next
      } catch (cause: unknown) {
        if (version !== requestVersion) return
        goal.value = null
        goalError.value = cause
      }
    }
  }

  // Saves are not optimistic: the backend emits journal:updated /
  // goal:updated and the listener re-pulls (docs/05 §5.5.5).
  async function saveJournal(entry: JournalDayDTO): Promise<void> {
    const day = context.value?.day
    if (day === undefined || journalSaving.value) return
    journalSaving.value = true
    journalError.value = null
    try {
      await saveJournalDay({ ...entry, day })
      // The event listener re-pulls; this re-pull is a safety net for the
      // degraded case where events are unavailable.
      await loadJournalAndGoal(day, requestVersion)
    } catch (cause: unknown) {
      journalError.value = cause
    } finally {
      journalSaving.value = false
    }
  }

  async function saveGoal(next: DayGoalDTO): Promise<void> {
    const day = context.value?.day
    if (day === undefined || goalSaving.value) return
    goalSaving.value = true
    goalError.value = null
    try {
      await saveDayGoal({ ...next, day })
      await loadJournalAndGoal(day, requestVersion)
    } catch (cause: unknown) {
      goalError.value = cause
    } finally {
      goalSaving.value = false
    }
  }

  // Regeneration is synchronous on the backend (one LLM call); the returned
  // recap is the stored result, so applying it directly is not an optimistic
  // update — the write already happened. The recap:updated listener re-pulls
  // for other views.
  async function regenerateRecap(): Promise<void> {
    const standupDay = context.value?.standupDay
    if (standupDay === undefined || recapGenerating.value) return
    if (!hasRecapGenerationBinding()) return
    recapGenerating.value = true
    recapGenerateError.value = null
    try {
      recap.value = await generateDailyRecap(standupDay)
    } catch (cause: unknown) {
      recapGenerateError.value = cause
    } finally {
      recapGenerating.value = false
    }
  }

  async function reloadRecap(standupDay: string, version: number): Promise<void> {
    try {
      const next = await getDailyRecap(standupDay)
      if (version !== requestVersion) return
      recap.value = next
    } catch (cause: unknown) {
      if (version !== requestVersion) return
      recapError.value = cause
    }
  }

  function startEvents(): void {
    if (stopEvents !== null) return
    stopEvents = onTimelineUpdated((updatedDay) => {
      if (updatedDay === null || updatedDay === context.value?.day) {
        void load(context.value?.day ?? '')
      }
    })
    stopJournalEvents = onJournalUpdated((updatedDay) => {
      if (updatedDay === null || updatedDay === context.value?.day) {
        const day = context.value?.day
        if (day !== undefined) void loadJournalAndGoal(day, requestVersion)
      }
    })
    stopGoalEvents = onGoalUpdated((updatedDay) => {
      if (updatedDay === null || updatedDay === context.value?.day) {
        const day = context.value?.day
        if (day !== undefined) void loadJournalAndGoal(day, requestVersion)
      }
    })
    stopRecapEvents = onRecapUpdated((updatedStandupDay) => {
      if (updatedStandupDay === null || updatedStandupDay === context.value?.standupDay) {
        const standupDay = context.value?.standupDay
        if (standupDay !== undefined) void reloadRecap(standupDay, requestVersion)
      }
    })
  }

  function stopListening(): void {
    stopEvents?.()
    stopJournalEvents?.()
    stopGoalEvents?.()
    stopRecapEvents?.()
    stopEvents = null
    stopJournalEvents = null
    stopGoalEvents = null
    stopRecapEvents = null
  }

  return {
    context,
    day,
    recap,
    capabilities,
    loading,
    error,
    recapUnavailable,
    recapError,
    recapGenerating,
    recapGenerateError,
    recapGenerationAvailable: computed(() => hasRecapGenerationBinding()),
    journal,
    journalUnavailable,
    journalError,
    journalSaving,
    goal,
    goalUnavailable,
    goalError,
    goalSaving,
    usingDevelopmentFixture,
    state,
    presentation,
    dayNavigationAvailable: computed(() => hasDailyDayBinding()),
    load,
    regenerateRecap,
    saveJournal,
    saveGoal,
    startEvents,
    stopListening,
  }
})
