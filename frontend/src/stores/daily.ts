import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  CapabilitiesDTO,
  CategoryDTO,
  DailyRecapDTO,
  DayContextDTO,
  TimelineCardDTO,
  TimelineDayDTO,
} from '@/api/dto'
import { getDailyDevelopmentFixture } from '@/api/developmentFixtures'
import {
  DailyUnavailableError,
  getDailyCapabilities,
  getDailyContext,
  getDailyRecap,
  getDailyTimeline,
  hasDailyDayBinding,
  hasDailyRecapBinding,
} from '@/api/daily'
import { onTimelineUpdated } from '@/api/timeline'

const SLOT_SECONDS = 15 * 60
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

export interface DailyPresentation {
  rows: DailyWorkflowRow[]
  ticks: DailyWorkflowTick[]
  metrics: DailyMetrics
  windowStartTs: number
  windowEndTs: number
  slotCount: number
}

export type DailyState = 'loading' | 'unavailable' | 'failure' | 'empty' | 'populated'

function categoryKey(name: string): string {
  return name.trim().toLocaleLowerCase()
}

function safeColor(value: string): string {
  return /^#[0-9a-f]{6}$/i.test(value) ? value : '#7D7A84'
}

function validCards(day: TimelineDayDTO): TimelineCardDTO[] {
  return day.cards
    .filter((card) => card.endTs > card.startTs)
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
    const isIdle = category?.isIdle ?? card.isIdle

    if (isIdle) distractedMinutes += durationMinutes
    else focusedMinutes += durationMinutes
    interruptions += card.distractions.length

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
  const usingDevelopmentFixture = ref(false)
  let requestVersion = 0
  let stopEvents: (() => void) | null = null

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
      } else {
        error.value = cause
      }
    } finally {
      if (version === requestVersion) loading.value = false
    }
  }

  function startEvents(): void {
    if (stopEvents !== null) return
    stopEvents = onTimelineUpdated((updatedDay) => {
      if (updatedDay === null || updatedDay === context.value?.day) {
        void load(context.value?.day ?? '')
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
    recap,
    capabilities,
    loading,
    error,
    recapUnavailable,
    recapError,
    usingDevelopmentFixture,
    state,
    presentation,
    dayNavigationAvailable: computed(() => hasDailyDayBinding()),
    load,
    startEvents,
    stopListening,
  }
})
