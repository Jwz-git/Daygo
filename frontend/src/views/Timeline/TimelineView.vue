<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import DevelopmentBadge from '@/components/DevelopmentBadge.vue'
import CalendarPopover from '@/components/CalendarPopover.vue'
import PageHeader from '@/components/PageHeader.vue'
import PeriodNav from '@/components/PeriodNav.vue'
import type { PlanBlockDTO, PlanBlockStatus, TimelineCardDTO, TimelineDayDTO } from '@/api/dto'
import { getTimelineDay } from '@/api/timeline'
import { calendarDayQuery, shiftCalendarDate } from '@/lib/calendarDate'
import { categoryLabel } from '@/lib/categoryLabel'
import { delayUntilDayContextRefresh } from '@/lib/dayContextRefresh'
import { formatTimelineForClipboard } from '@/lib/timelineClipboard'
import { formatTimeZoneName } from '@/lib/timeFormat'
import { safeTimeZone } from '@/lib/timeZone'
import { useDailyStore } from '@/stores/daily'
import { usePlanStore } from '@/stores/plan'
import { useRecordingStore } from '@/stores/recording'
import { useTimelineStore } from '@/stores/timeline'
import { getReviewTotals } from '@/api/review'
import CardReviewFlow from './CardReviewFlow.vue'
import CategoryManagerModal from './CategoryManagerModal.vue'
import { ZERO_REVIEW_TOTALS, type ReviewTotals } from './review'
import PlanBlockPopover from './PlanBlockPopover.vue'
import TimelineInspector from './TimelineInspector.vue'
import TimelineStatePanel from './TimelineStatePanel.vue'
import TimelineTrack from './TimelineTrack.vue'
import TimelineWeekView from './TimelineWeekView.vue'
import { cardRegenerationFailureKey } from './cardRegenerationFailure'
import { buildWeekColumns } from './weekLayout'
import { safeCategoryColor } from './layout'

const timeline = useTimelineStore()
// The inspector's default pane embeds the day-goal form; the daily store owns
// that state (bindings, events, save path) for both pages.
const daily = useDailyStore()
// The displayed day's plan: the inspector panel edits it, the track draws it.
const plan = usePlanStore()
const planBlocks = computed(() => (plan.day === context.value?.day ? plan.blocks : []))
// Recording state is shared with the rail's RecordingControl; this view only
// reads it and triggers actions, and must not stop the shared event listener
// on unmount (the rail outlives this page).
const recording = useRecordingStore()
const {
  context,
  day,
  cards,
  state,
  selectedCard,
  selectedCardID,
  selectedFailure,
  selectedFailureTs,
  categoryFilter,
  capabilities,
  dayNavigationAvailable,
  usingDevelopmentFixture,
  pendingAction,
  pendingCardID,
  actionError,
  failedAction,
  failedCardID,
  actionAvailability,
} = storeToRefs(timeline)
const { locale, t } = useI18n()
const cardReprocessFailureKey = computed(() =>
  failedAction.value === 'reprocess-card' && failedCardID.value !== null && actionError.value !== null
    ? cardRegenerationFailureKey(actionError.value)
    : null,
)
const generalActionFailed = computed(() => actionError.value !== null && failedAction.value !== 'reprocess-card')
const route = useRoute()
const router = useRouter()
const copyState = ref<'idle' | 'copied' | 'failed'>('idle')
const showCategoryManager = ref(false)
let dayRefreshTimer: number | null = null

/*
 * Day/week view mode. Week mode shows the calendar week (Monday-based) that
 * contains the selected day; the 04:00 logical-day rule still comes from each
 * day's own GetTimelineDay window, never from frontend math.
 */
type ViewMode = 'day' | 'week'
const viewMode = ref<ViewMode>('day')
const weekDays = ref<(Awaited<ReturnType<typeof getTimelineDay>> | null)[]>([])
const weekLoading = ref(false)
let weekRequestVersion = 0

const ISO_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/

/** Monday of the calendar week containing the ISO day. */
function mondayOf(iso: string): string | null {
  const match = ISO_PATTERN.exec(iso)
  if (match === null) return null
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])))
  if (Number.isNaN(date.getTime())) return null
  const offset = (date.getUTCDay() + 6) % 7
  return shiftCalendarDate(iso, -offset)
}

const weekKeys = computed<string[]>(() => {
  const anchor = context.value?.day ?? routeDay()
  const monday = mondayOf(anchor)
  if (monday === null) return []
  return Array.from({ length: 7 }, (_, index) => shiftCalendarDate(monday, index) ?? '')
})

async function loadWeek(options: { silent?: boolean } = {}): Promise<void> {
  const keys = weekKeys.value
  if (keys.length === 0) return
  const version = ++weekRequestVersion
  // A silent reload keeps the current columns rendered while refetching —
  // only a first load (nothing to show yet) turns on the loading note.
  if (!options.silent || weekDays.value.every((entry) => entry === null)) {
    weekLoading.value = true
  }
  const results = await Promise.allSettled(keys.map((key) => getTimelineDay(key)))
  if (version !== weekRequestVersion) return
  weekDays.value = results.map((result) => (result.status === 'fulfilled' ? result.value : null))
  weekLoading.value = false
}

const weekColumns = computed(() => {
  const format = new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit' })
  return buildWeekColumns(weekDays.value, categoryFilter.value, format, pendingCardID.value)
})

const weekTitle = computed(() => {
  const keys = weekKeys.value
  if (keys.length === 0) return ''
  const format = new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric' })
  const first = keys[0]
  const last = keys[keys.length - 1]
  if (first === '' || last === '') return ''
  return `${format.format(new Date(`${first}T00:00:00`))} – ${format.format(new Date(`${last}T00:00:00`))}`
})

const showPauseButton = computed(
  () => recording.lifecycle === 'capturing' || recording.lifecycle === 'paused',
)

/*
 * The placeholder card sits at the current time while the displayed day is the
 * live logical day and recording is live (nine-cell wave) or paused (hold icon
 * + resume hint) — idle recording has no next card coming. "Live logical day"
 * comes from the backend-owned day window, so an explicit ?day=today shows the
 * placeholder too; only the window check decides, not the route shape.
 */
type GenState = 'off' | 'capturing' | 'paused'
const generating = computed<GenState>(() => {
  const current = context.value
  if (current === null) return 'off'
  const now = Math.floor(Date.now() / 1000)
  if (now < current.dayStartTs || now >= current.dayEndTs) return 'off'
  if (recording.lifecycle === 'capturing' || recording.lifecycle === 'starting') return 'capturing'
  if (recording.lifecycle === 'paused') return 'paused'
  return 'off'
})

// Clicking the paused card resumes recording, like the header's pause pill.
function resumeRecording(): void {
  if (recording.lifecycle !== 'paused' || recording.pendingAction !== null) return
  void recording.perform('resume')
}

function selectDayFromWeek(dayKey: string): void {
  if (dayKey === '') return
  void router.push({ name: 'timeline', query: { ...route.query, day: dayKey } })
}

const showCalendar = ref(false)

function pickDate(dayKey: string): void {
  showCalendar.value = false
  if (dayKey === '') return
  void router.push({ name: 'timeline', query: { ...route.query, day: dayKey } })
}

/*
 * Review flow: the queue is the day's activity cards the user has not judged
 * in this session (idle/System cards never enter it). Judgments persist via
 * UpdateCardCategory inside the flow; the set here only tracks the badge and
 * queue membership.
 */
const showReview = ref(false)
const reviewedIds = ref<ReadonlySet<number>>(new Set())

const reviewQueue = computed<TimelineCardDTO[]>(() =>
  (day.value?.cards ?? []).filter(
    (card) => card.day === day.value?.day && !card.isIdle && card.category !== 'System' && !reviewedIds.value.has(card.id),
  ),
)

function onJudged(cardID: number, removed: boolean): void {
  const next = new Set(reviewedIds.value)
  if (removed) next.delete(cardID)
  else next.add(cardID)
  reviewedIds.value = next
}

/*
 * Session review totals for the inspector's "你的回顾" panel. The review flow
 * is the single writer; the inspector only renders them.
 */
const reviewTotals = ref<ReviewTotals>({ ...ZERO_REVIEW_TOTALS })

/*
 * Totals are persisted per logical day; reload them whenever the displayed
 * day changes so reopening the review flow or the inspector shows the stored
 * split rather than a stale session snapshot.
 */
function refreshReviewTotals(day: string): void {
  void getReviewTotals(day)
    .then((totals) => {
      reviewTotals.value = totals
      // Verdicts persist across restarts: the already-judged cards re-enter
      // neither the badge count nor the review queue.
      reviewedIds.value = new Set(totals.reviewedCardIds ?? [])
    })
    .catch(() => { reviewTotals.value = { ...ZERO_REVIEW_TOTALS } })
}

watch(context, (current) => {
  if (current === null) return
  refreshReviewTotals(current.day)
})

// A per-card verdict edit from the inspector re-reads the day totals so the
// "你的回顾" panel and the review queue reflect the change on the next open.
function onVerdictChanged(): void {
  if (context.value === null) return
  refreshReviewTotals(context.value.day)
}

async function onCategoriesSaved(): Promise<void> {
  showCategoryManager.value = false
  // Names may have been rewritten on cards; refetch the day (and week when
  // it is visible) so every surface reflects the new set.
  await timeline.load(routeDay())
  if (viewMode.value === 'week') await loadWeek({ silent: true })
}

async function closeReview(): Promise<void> {
  showReview.value = false
  // Categories may have changed; refetch so the track and inspector agree.
  await timeline.load(routeDay())
}

/*
 * Week-mode card detail: selecting a week card opens the same inspector pane
 * with the card's own day DTO. Closing/acting refreshes the week columns.
 */
const weekSelection = ref<{ day: TimelineDayDTO; card: TimelineCardDTO } | null>(null)

function openWeekCard(dayKey: string, cardId: number): void {
  const dayDTO = weekDays.value.find((entry) => entry !== null && entry.day === dayKey)
  const card = dayDTO?.cards.find((entry) => entry.id === cardId)
  if (dayDTO === undefined || dayDTO === null || card === undefined) return
  weekSelection.value = { day: dayDTO, card }
}

function closeWeekCard(): void {
  weekSelection.value = null
  // Wait for the grid-expand transition to finish before refreshing columns;
  // a mid-transition data swap re-lays-out every card and reads as a flash.
  window.setTimeout(() => { void loadWeek({ silent: true }) }, 450)
}

function anchorDay(dayKey: string): void {
  if (dayKey === '') return
  weekSelection.value = null
  void router.push({ name: 'timeline', query: { ...route.query, day: dayKey } })
}

async function saveWeekEdits(
  cardID: number,
  edits: { title?: string; category?: string; summary?: string; detailedSummary?: string },
): Promise<void> {
  await timeline.saveCardEdits(cardID, edits)
  await loadWeek({ silent: true })
  if (weekSelection.value !== null) {
    const card = weekSelection.value.day.cards.find((entry) => entry.id === cardID)
    if (card !== undefined) weekSelection.value = { day: weekSelection.value.day, card }
  }
}

async function deleteWeekCard(cardID: number): Promise<void> {
  await timeline.removeCard(cardID)
  weekSelection.value = null
  window.setTimeout(() => { void loadWeek({ silent: true }) }, 450)
}

async function reprocessWeekCard(cardID: number): Promise<void> {
  const ok = await timeline.reprocessCard(cardID)
  // The rewrite is synchronous, so the new cards are already stored once this
  // resolves; re-pull the week to draw them where the old ones were.
  if (ok) await loadWeek({ silent: true })
}

watch(viewMode, (mode) => {
  if (mode === 'day') weekSelection.value = null
})

/*
 * The plan block popover, opened from the day track or the week grid. It
 * tracks the block by id so a status change re-renders it from the store's
 * re-pulled data instead of a stale copy.
 */
const planPopover = ref<{ id: number; day: string; anchor: DOMRect } | null>(null)

const planPopoverBlock = computed<PlanBlockDTO | null>(() => {
  const open = planPopover.value
  if (open === null) return null
  const pool = plan.day === open.day ? plan.blocks : (plan.week[open.day] ?? [])
  return pool.find((block) => block.id === open.id) ?? plan.week[open.day]?.find((block) => block.id === open.id) ?? null
})

function openPlanPopover(block: PlanBlockDTO, anchor: DOMRect): void {
  planPopover.value = planPopover.value?.id === block.id ? null : { id: block.id, day: block.day, anchor }
}

function setPlanStatus(status: PlanBlockStatus): void {
  const open = planPopover.value
  if (open !== null) void plan.setStatus(open.id, status)
}

// Editing belongs to the inspector's plan panel: show the block's day with
// nothing selected, then ask the panel to open that block's form.
function editPlanBlock(): void {
  const open = planPopover.value
  planPopover.value = null
  if (open === null) return
  weekSelection.value = null
  timeline.selectCard(null)
  if (viewMode.value === 'week') viewMode.value = 'day'
  if (context.value?.day !== open.day) {
    void router.push({ name: 'timeline', query: { ...route.query, day: open.day } })
  }
  plan.requestFocus(open.id, true)
}

watch([viewMode, () => context.value?.day], () => { planPopover.value = null })
const dateTitle = computed(() => {
  if (viewMode.value === 'week' && weekTitle.value !== '') return weekTitle.value
  if (context.value === null) return t('timeline.title')
  return new Intl.DateTimeFormat(locale.value, {
    weekday: 'short', month: 'short', day: 'numeric', timeZone: safeTimeZone(context.value.timeZone),
  }).format(new Date(context.value.dayStartTs * 1000))
})

const localizedTimeZone = computed(() =>
  formatTimeZoneName(context.value?.timeZone, locale.value),
)

const filterCategories = computed(() =>
  (day.value?.categories ?? []).filter((category) => !category.isSystem),
)

/*
 * Per-category usage for the manager modal: card count and tracked minutes
 * from the current day. Rename / recolor / create have no backend binding
 * yet, so the manager is a read-only overview whose rows double as filter
 * shortcuts; usage-descending order puts the day's real work on top.
 */
const managerRows = computed(() => {
  const stats = new Map<string, { count: number; minutes: number }>()
  for (const card of day.value?.cards ?? []) {
    if (card.category === 'System') continue
    const entry = stats.get(card.category) ?? { count: 0, minutes: 0 }
    entry.count += 1
    entry.minutes += Math.max(0, Math.round((card.endTs - card.startTs) / 60))
    stats.set(card.category, entry)
  }
  return filterCategories.value
    .map((category) => ({ category, stat: stats.get(category.name) }))
    .sort(
      (a, b) =>
        (b.stat?.minutes ?? 0) - (a.stat?.minutes ?? 0) ||
        a.category.sortOrder - b.category.sortOrder,
    )
})

function pickManagerCategory(name: string): void {
  timeline.setCategoryFilter(name)
  showCategoryManager.value = false
}

const hasTrack = computed(() =>
  day.value !== null && ['populated', 'processing', 'failure', 'empty'].includes(state.value),
)

const canNavigateBackward = computed(
  () => dayNavigationAvailable.value && context.value !== null,
)

const canNavigateForward = computed(
  () =>
    dayNavigationAvailable.value &&
    context.value !== null &&
    context.value.nowTs >= context.value.dayEndTs,
)

function routeDay(): string {
  return calendarDayQuery(route.query.day)
}

function navigate(offset: -1 | 1): void {
  const current = context.value
  if (current === null) return
  const target = shiftCalendarDate(current.day, offset)
  if (target === null) return
  void router.push({ name: 'timeline', query: { ...route.query, day: target } })
}

function goToToday(): void {
  if (route.query.day === undefined) {
    void timeline.load()
    return
  }
  const query = { ...route.query }
  delete query.day
  void router.push({ name: 'timeline', query })
}

function isFollowingToday(): boolean {
  return route.query.day === undefined
}

function stopDayRefresh(): void {
  if (dayRefreshTimer !== null) window.clearTimeout(dayRefreshTimer)
  dayRefreshTimer = null
}

function scheduleDayRefresh(): void {
  stopDayRefresh()
  const current = context.value
  if (!isFollowingToday() || current === null) return

  // dayEndTs comes from GetDayContext: it preserves the backend-owned 04:00
  // boundary and handles local time-zone/DST rules without frontend guesses.
  dayRefreshTimer = window.setTimeout(() => {
    dayRefreshTimer = null
    void timeline.load()
  }, delayUntilDayContextRefresh(current.dayEndTs) + 50)
}

function refreshWhenWindowReturns(): void {
  if (document.visibilityState === 'visible' && isFollowingToday()) {
    void timeline.load('', { silent: true })
  }
}

async function copyTimeline(): Promise<void> {
  if (day.value === null || cards.value.length === 0) return
  try {
    await navigator.clipboard.writeText(formatTimelineForClipboard(day.value.day, cards.value, t))
    copyState.value = 'copied'
  } catch {
    copyState.value = 'failed'
  }
  window.setTimeout(() => { copyState.value = 'idle' }, 1600)
}

async function reprocessCurrentDay(): Promise<void> {
  if (context.value === null) return
  await timeline.reprocessCurrentDay(context.value.day)
}

onMounted(() => {
  timeline.startEvents()
  daily.startEvents()
  plan.startListening()
  recording.startListening()
  window.addEventListener('focus', refreshWhenWindowReturns)
  document.addEventListener('visibilitychange', refreshWhenWindowReturns)
})
watch(() => route.query.day, () => { void timeline.load(routeDay()) }, { immediate: true })
watch([viewMode, weekKeys], () => {
  if (viewMode.value !== 'week') return
  void loadWeek()
  void plan.loadWeek(weekKeys.value)
}, { immediate: true })
watch(() => context.value?.day, (day) => {
  if (day !== undefined) {
    void daily.load(day)
    void plan.load(day)
  }
}, { immediate: true })
watch([() => route.query.day, () => context.value?.dayEndTs], scheduleDayRefresh, { immediate: true })
onBeforeUnmount(() => {
  stopDayRefresh()
  window.removeEventListener('focus', refreshWhenWindowReturns)
  document.removeEventListener('visibilitychange', refreshWhenWindowReturns)
  timeline.stopListening()
  daily.stopListening()
  plan.stopListening()
})
</script>

<template>
  <div class="page timeline-page" @keydown.esc="timeline.selectCard(null)">
    <PageHeader :title="dateTitle" hide-title>
      <template #lead>
        <PeriodNav
          :label="t('timeline.navigation.label')"
          :backward-title="dayNavigationAvailable ? t('common.action.previous') : t('timeline.navigation.backendRequired')"
          :forward-title="!dayNavigationAvailable ? t('timeline.navigation.backendRequired') : canNavigateForward ? t('common.action.next') : t('timeline.navigation.futureUnavailable')"
          :can-backward="canNavigateBackward"
          :can-forward="canNavigateForward"
          :current-label="t('common.action.today')"
          :current-disabled="!dayNavigationAvailable && !usingDevelopmentFixture"
          @navigate="navigate"
          @current="goToToday"
        />
        <div class="header-tools">
          <div class="calendar-anchor">
            <button
              type="button"
              class="tool-button"
              data-calendar-toggle
              :title="t('timeline.calendar.open')"
              :aria-label="t('timeline.calendar.open')"
              :aria-expanded="showCalendar"
              @click="showCalendar = !showCalendar"
            >
              <DgIcon name="calendar" :size="16" />
            </button>
            <Transition name="calendar-pop">
              <CalendarPopover
                v-if="showCalendar"
                :selected="context?.day ?? routeDay()"
                @select="pickDate"
                @close="showCalendar = false"
              />
            </Transition>
          </div>
          <div class="mode-toggle" role="tablist" :aria-label="t('timeline.mode.label')">
            <button
              v-for="mode in (['day', 'week'] as const)"
              :key="mode"
              type="button"
              class="mode-toggle__item"
              role="tab"
              :aria-selected="viewMode === mode"
              :class="{ 'is-active': viewMode === mode }"
              @click="viewMode = mode"
            >
              {{ t(`timeline.mode.${mode}`) }}
            </button>
          </div>
          <h1 class="timeline-date dg-page-date dg-display">{{ dateTitle }}</h1>
        </div>
      </template>

      <template #trail>
        <button
          v-if="showPauseButton"
          type="button"
          class="pause-pill"
          :disabled="recording.pendingAction !== null"
          :title="recording.lifecycle === 'paused' ? t('recording.action.resume') : t('recording.action.pause')"
          @click="recording.perform(recording.lifecycle === 'paused' ? 'resume' : 'pause')"
        >
          <DgIcon v-if="recording.lifecycle !== 'paused'" name="pause" :size="11" />
          <DgIcon v-else name="play" :size="11" />
          <span>{{ recording.lifecycle === 'paused' ? t('recording.action.resume') : t('recording.action.pause') }}</span>
        </button>
        <DevelopmentBadge v-if="usingDevelopmentFixture">{{ t('timeline.developmentFixture') }}</DevelopmentBadge>
        <div v-if="day && localizedTimeZone" class="day-meta">
          <span>{{ localizedTimeZone }}</span>
        </div>
      </template>
    </PageHeader>

    <div class="filter-bar" :aria-label="t('timeline.filter.label')">
      <button
        type="button"
        class="filter-chip"
        :class="{ 'is-selected': categoryFilter === null }"
        @click="timeline.setCategoryFilter(null)"
      >
        {{ t('timeline.filter.all') }}
      </button>
      <button
        v-for="category in filterCategories"
        :key="category.id"
        type="button"
        class="filter-chip"
        :class="{ 'is-selected': categoryFilter === category.name }"
        @click="timeline.setCategoryFilter(category.name)"
      >
        <i :style="{ background: safeCategoryColor(category.colorHex) }" aria-hidden="true"></i>
        {{ categoryLabel(category.name, t) }}
      </button>
      <span class="filter-bar__spacer"></span>
      <button
        type="button"
        class="filter-manage filter-edit"
        :title="t('timeline.manage2.open')"
        :aria-label="t('timeline.manage2.open')"
        :disabled="!actionAvailability.manageCategories"
        @click="showCategoryManager = true"
      >
        <DgIcon name="pencil" :size="14" />
      </button>
      <span v-if="generalActionFailed && selectedCard === null" class="filter-error" role="alert">
        {{ t('timeline.actionFailed') }}
      </span>
    </div>

    <!--
      Day and week each own their full layout and share one grid cell, so the
      switch crossfades both at once (Dayflow: day scales 0.95 ↔ 1, week
      1.05 ↔ 1, 240ms ease-out-quart) and neither pane is ever squeezed into
      the other's columns mid-switch.
    -->
    <div class="timeline-body">
      <Transition name="mode-week">
        <div
          v-if="viewMode === 'week'"
          key="week"
          class="timeline-body__week-pane"
          :class="{ 'has-inspector': weekSelection !== null }"
        >
          <TimelineWeekView
            class="timeline-body__week"
            :day-keys="weekKeys"
            :columns="weekColumns"
            :selected-day="context?.day ?? ''"
            :selected-card-id="weekSelection?.card.id ?? null"
            :week-loading="weekLoading"
            :generating="generating"
            :plan-blocks="plan.week"
            :open-plan-id="planPopover?.id ?? null"
            @open-plan="openPlanPopover"
            @anchor-day="selectDayFromWeek"
            @select-card="openWeekCard"
          />
          <Transition name="inspector">
            <TimelineInspector
              v-if="weekSelection !== null"
              class="timeline-body__inspector"
              :day="weekSelection.day"
              :time-zone="context?.timeZone ?? 'UTC'"
              :card="weekSelection.card"
              :failure="null"
              :can-write="capabilities?.canWrite ?? false"
              :actions="actionAvailability"
              :pending-action="pendingAction"
              :action-failed="generalActionFailed"
              :card-reprocess-failure-key="weekSelection?.card.id === failedCardID ? cardReprocessFailureKey : null"
              :goal="daily.goal"
              :goal-unavailable="daily.goalUnavailable"
              :goal-failed="daily.goalError !== null"
              :goal-saving="daily.goalSaving"
              :review-totals="reviewTotals"
              @close="closeWeekCard"
              @save-edits="saveWeekEdits"
              @delete="deleteWeekCard"
              @reprocess-card="reprocessWeekCard"
              @retry="timeline.retryFailure"
              @stop-retries="timeline.stopFailureRetries"
              @dismiss-failure="timeline.dismissFailure"
              @reprocess="reprocessCurrentDay"
              @save-goal="daily.saveGoal"
              @verdict-changed="onVerdictChanged"
            />
          </Transition>
        </div>
      </Transition>
      <Transition name="mode-day">
        <div v-if="viewMode === 'day'" key="day" class="timeline-body__day">
          <TimelineStatePanel
            v-if="!hasTrack"
            class="timeline-body__state"
            :state="state"
            @retry="timeline.load(context?.day ?? '')"
          />

          <template v-else-if="day && context">
            <TimelineTrack
              class="timeline-body__track"
              :context="context"
              :cards="cards"
              :categories="day.categories"
              :failures="day.failures"
              :processing-ranges="day.processingRanges"
              :selected-card-i-d="selectedCardID"
              :selected-failure-ts="selectedFailureTs"
              :regenerating-card-i-d="pendingCardID"
              :generating="generating"
              :plan-blocks="planBlocks"
              :open-plan-id="planPopover?.id ?? null"
              @open-plan="openPlanPopover"
              @resume="resumeRecording"
              @select="timeline.selectCard"
              @select-failure="timeline.selectFailure"
              @clear="timeline.selectCard(null)"
            />
            <TimelineInspector
              class="timeline-body__inspector"
              :class="{ 'has-selection': selectedCard !== null || selectedFailure !== null }"
              :day="day"
              :time-zone="context.timeZone"
              :card="selectedCard"
              :failure="selectedFailure"
              :can-write="capabilities?.canWrite ?? false"
              :actions="actionAvailability"
              :pending-action="pendingAction"
              :action-failed="generalActionFailed"
              :card-reprocess-failure-key="selectedCard?.id === failedCardID ? cardReprocessFailureKey : null"
              :goal="daily.goal"
              :goal-unavailable="daily.goalUnavailable"
              :goal-failed="daily.goalError !== null"
              :goal-saving="daily.goalSaving"
              :review-totals="reviewTotals"
              @close="timeline.selectCard(null)"
              @save-edits="(cardID, edits) => timeline.saveCardEdits(cardID, edits)"
              @delete="timeline.removeCard"
              @reprocess-card="timeline.reprocessCard"
              @retry="timeline.retryFailure"
              @stop-retries="timeline.stopFailureRetries"
              @dismiss-failure="timeline.dismissFailure"
              @reprocess="reprocessCurrentDay"
              @save-goal="daily.saveGoal"
              @verdict-changed="onVerdictChanged"
            />

            <!--
              Timeline footer (Dayflow MainView.timelineFooter): the review
              badge centred under the track while cards await review, the
              secondary "copy timeline" button at the track's bottom right.
              Living inside the day pane, both fade with the mode switch.
            -->
            <div class="timeline-footer">
              <Transition name="footer-pop">
                <button
                  v-if="reviewQueue.length > 0"
                  type="button"
                  class="review-badge"
                  :title="t('timeline.review.title')"
                  :aria-label="`${reviewQueue.length} · ${t('timeline.review.action')}`"
                  @click="showReview = true"
                >
                  <span class="review-badge__stack" aria-hidden="true">
                    <i class="review-badge__back"></i>
                    <i class="review-badge__front">{{ reviewQueue.length }}</i>
                  </span>
                  <span>{{ t('timeline.review.action') }}</span>
                </button>
              </Transition>
              <button
                type="button"
                class="copy-button"
                :class="{ 'is-copied': copyState === 'copied', 'is-failed': copyState === 'failed' }"
                :disabled="cards.length === 0"
                :title="t('timeline.copy.action')"
                @click="copyTimeline"
              >
                <Transition name="copy-swap" mode="out-in">
                  <span :key="copyState" class="copy-button__content">
                    <DgIcon v-if="copyState === 'idle'" name="copy" :size="12" />
                    <DgIcon v-else-if="copyState === 'copied'" name="check" :size="12" />
                    <DgIcon v-else name="close" :size="12" />
                    {{ copyState === 'copied' ? t('timeline.copy.copied') : copyState === 'failed' ? t('timeline.copy.failed') : t('timeline.copy.action') }}
                  </span>
                </Transition>
              </button>
            </div>
          </template>
        </div>
      </Transition>
    </div>

    <PlanBlockPopover
      v-if="planPopover !== null && planPopoverBlock !== null"
      :key="planPopover.id"
      :block="planPopoverBlock"
      :anchor="planPopover.anchor"
      :can-write="(capabilities?.canWrite ?? false) && plan.available"
      :pending="plan.pending"
      @close="planPopover = null"
      @status="setPlanStatus"
      @edit="editPlanBlock"
    />

    <!-- Category manager wizard -->
    <Teleport to="body">
      <div v-if="showCategoryManager" class="modal-backdrop" @click.self="showCategoryManager = false">
        <CategoryManagerModal
          v-if="day !== null"
          :categories="day.categories"
          :can-write="capabilities?.canWrite ?? false"
          @close="showCategoryManager = false"
          @saved="onCategoriesSaved"
        />
      </div>
    </Teleport>

    <!-- Review flow modal -->
    <Teleport to="body">
      <div v-if="showReview" class="modal-backdrop" @click.self="closeReview">
        <CardReviewFlow
          v-if="day !== null"
          :day="day"
          :cards="reviewQueue"
          :time-zone="context?.timeZone ?? 'UTC'"
          :initial-totals="reviewTotals"
          @close="closeReview"
          @judged="onJudged"
          @totals="reviewTotals = $event"
        />
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.day-meta {
  display: flex;
  align-items: center;
  gap: 9px;
  color: var(--dg-text-muted);
  font-size: 10px;
  white-space: nowrap;
}


.filter-bar {
  display: flex;
  align-items: center;
  gap: 7px;
  min-height: 43px;
  padding: 0 var(--dg-page-padding) 13px;
  overflow-x: auto;
}

.filter-chip,
.filter-manage {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 6px;
  min-height: 26px;
  padding: 3px 11px;
  border: none;
  border-radius: 999px;
  background: var(--dg-capsule-fill);
  color: var(--dg-capsule-text);
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  transition: background var(--dg-motion-fast) ease, border-color var(--dg-motion-fast) ease;
}

.filter-chip:hover { background: var(--dg-capsule-hover); }
.filter-chip:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.filter-chip.is-selected {
  background: var(--dg-capsule-selected-fill);
  color: var(--dg-capsule-selected-text);
}
.filter-chip i { width: 7px; height: 7px; border-radius: 50%; }
.filter-bar__spacer { flex: 1; }
/* Hover feedback is fill + colour only — no transform or shadow lift, so the
   chip never floats out of the bar. */
.filter-manage { color: var(--dg-text-muted); }

/* Pencil entry to the category wizard. */
.filter-edit {
  width: 28px;
  height: 28px;
  background: transparent;
  justify-content: center;
  padding: 0;
}

.filter-edit:not(:disabled) { color: var(--dg-text-secondary); cursor: pointer; }
.filter-edit:not(:disabled):hover { color: var(--dg-accent); background: var(--dg-hover-fill); }
.filter-edit:not(:disabled):focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.filter-edit svg { width: 14px; height: 14px; }
.filter-manage:not(:disabled) { color: var(--dg-text-secondary); cursor: pointer; }
.filter-manage:not(:disabled):hover { background: var(--dg-capsule-hover); color: var(--dg-text-primary); }
.filter-manage:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.filter-error { flex: none; color: var(--dg-danger); font-size: 10px; white-space: nowrap; }

.timeline-body {
  position: relative;
  display: grid;
  /* One stacking cell: the day and week panes overlap while they crossfade. */
  grid-template-areas: 'pane';
  grid-template-columns: minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
  flex: 1;
  min-height: 0;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

.timeline-body__day,
.timeline-body__week-pane {
  display: grid;
  grid-area: pane;
  grid-template-rows: minmax(0, 1fr);
  grid-template-columns: minmax(0, 1fr) var(--dg-inspector-width);
  gap: var(--dg-inspector-gap);
  min-width: 0;
  min-height: 0;
}

/* The week grid grows over the inspector when no card is selected and
   shrinks back on selection; grid-template-columns animates in the engines
   that support it and jumps elsewhere. */
.timeline-body__week-pane {
  grid-template-columns: minmax(0, 1fr) 0px;
  column-gap: 0px;
  transition:
    grid-template-columns var(--dg-motion-slow) var(--dg-ease-glide),
    column-gap var(--dg-motion-slow) var(--dg-ease-glide);
}

.timeline-body__week-pane.has-inspector {
  grid-template-columns: minmax(0, 1fr) var(--dg-inspector-width);
  column-gap: var(--dg-inspector-gap);
}

.timeline-body__week { min-width: 0; min-height: 0; }
.timeline-body__state { grid-column: 1 / -1; }
.timeline-body__track,
.timeline-body__inspector { min-width: 0; }

/* Explicit cells: the footer overlays the track's cell. */
.timeline-body__day > .timeline-body__track { grid-row: 1; grid-column: 1; }
.timeline-body__day > .timeline-body__inspector { grid-row: 1; grid-column: 2; }

/*
 * Day/week switch, after Dayflow's MainView: both panes animate at once.
 * Day is the zoomed-in view (grows 0.95 → 1 as it arrives, shrinks back as
 * it leaves); week is the zoomed-out one (settles 1.05 → 1, grows away).
 * ease-out-quart over 240ms; the arriving pane sits on top.
 */
.mode-day-enter-active,
.mode-day-leave-active,
.mode-week-enter-active,
.mode-week-leave-active {
  transition:
    opacity 240ms cubic-bezier(0.165, 0.84, 0.44, 1),
    transform 240ms cubic-bezier(0.165, 0.84, 0.44, 1);
}

.mode-day-enter-active,
.mode-week-enter-active { z-index: 1; }

.mode-day-leave-active,
.mode-week-leave-active { z-index: 0; pointer-events: none; }

.mode-day-enter-from,
.mode-day-leave-to {
  opacity: 0;
  transform: scale(0.95);
}

.mode-week-enter-from,
.mode-week-leave-to {
  opacity: 0;
  transform: scale(1.05);
}

@media (prefers-reduced-motion: reduce) {
  .mode-day-enter-active,
  .mode-day-leave-active,
  .mode-week-enter-active,
  .mode-week-leave-active {
    transition: opacity 80ms linear;
  }

  .mode-day-enter-from,
  .mode-day-leave-to,
  .mode-week-enter-from,
  .mode-week-leave-to {
    transform: none;
  }
}

/* Inspector pane sliding open / closed. */
.inspector-enter-active,
.inspector-leave-active {
  transition:
    opacity 200ms ease,
    transform var(--dg-motion-base) var(--dg-ease-glide);
}

.inspector-enter-from,
.inspector-leave-to {
  opacity: 0;
  transform: scale(0.985) translateX(14px);
}

.header-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}

.calendar-anchor {
  position: relative;
}

/* Calendar open/close: a small drop-scale from the anchor corner. */
.calendar-pop-enter-active,
.calendar-pop-leave-active {
  transition:
    opacity 160ms ease,
    transform 200ms var(--dg-ease-glide);
  transform-origin: top left;
}

.calendar-pop-enter-from,
.calendar-pop-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-6px);
}

.tool-button {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--dg-text-secondary);
  cursor: pointer;
  transition: background var(--dg-motion-fast) ease, transform var(--dg-motion-base) var(--dg-ease-glide);
}

.tool-button:hover { background: var(--dg-capsule-hover); color: var(--dg-text-primary); }
.tool-button:active { transform: scale(0.96); }
.tool-button:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.tool-button svg { width: 16px; height: 16px; }

/* In-header date, sitting right of the controls like the reference. */
.timeline-date {
  margin-left: 8px;
}

.mode-toggle {
  display: inline-flex;
  padding: 2px;
  border-radius: 8px;
  background: var(--dg-capsule-fill);
}

.mode-toggle__item {
  min-width: 44px;
  padding: 2px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition:
    background var(--dg-motion-fast) ease,
    color var(--dg-motion-fast) ease,
    box-shadow var(--dg-motion-fast) ease;
}

.mode-toggle__item:hover { color: var(--dg-text-primary); }
.mode-toggle__item:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.mode-toggle__item.is-active {
  background: var(--dg-segment-thumb);
  color: var(--dg-text-primary);
  box-shadow: var(--dg-segment-thumb-shadow);
}

/* Warm pill echoing the reference recording control; kept local because no
   palette token carries the warm recording hue. */
.pause-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  height: 34px;
  padding: 0 15px;
  border: none;
  border-radius: 999px;
  background: rgba(232, 128, 74, 0.2);
  color: #b25a22;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--dg-motion-fast) ease, transform var(--dg-motion-base) var(--dg-ease-glide);
}

:root[data-dg-appearance='dark'] .pause-pill {
  background: rgba(232, 128, 74, 0.16);
  color: #eda06c;
}

.pause-pill svg { width: 11px; height: 11px; }
.pause-pill:hover:not(:disabled) { background: rgba(232, 128, 74, 0.3); }
.pause-pill:active:not(:disabled) { transform: scale(0.96); }
.pause-pill:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.pause-pill:disabled { opacity: 0.55; cursor: default; }

@media (max-width: 1000px) {
  .timeline-body__day,
  .timeline-body__week-pane,
  .timeline-body__week-pane.has-inspector { grid-template-columns: minmax(0, 1fr); column-gap: 0px; }
  .timeline-body__inspector { display: none; }
  .timeline-body__inspector.has-selection {
    position: absolute;
    z-index: 9;
    top: 12px;
    right: 12px;
    bottom: 12px;
    display: block;
    width: min(var(--dg-inspector-width), calc(100% - 24px));
    border: 1px solid var(--dg-panel-border);
    border-radius: var(--dg-panel-radius);
    background: var(--dg-glass-fallback);
    box-shadow: var(--dg-glass-shadow);
  }

  /* Mirrors the .dg-panel glass in base.css; kept local because the class
     itself cannot be conditional on this breakpoint. */
  @supports ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
    .timeline-body__inspector.has-selection {
      background: var(--dg-glass-tint);
      -webkit-backdrop-filter: blur(var(--dg-glass-blur)) saturate(var(--dg-glass-saturation));
      backdrop-filter: blur(var(--dg-glass-blur)) saturate(var(--dg-glass-saturation));
    }
  }
}

@media (max-width: 720px) {
  .day-meta { display: none; }
  .timeline-body { padding-right: 16px; padding-left: 16px; }
  .filter-bar { padding-right: 16px; padding-left: 16px; }
}

/*
 * Timeline footer, after Dayflow's MainView.timelineFooter: an overlay on
 * the track's grid cell, 24px in and 17px up. Only the buttons take clicks.
 */
.timeline-footer {
  position: relative;
  z-index: 5;
  display: flex;
  grid-row: 1;
  grid-column: 1;
  align-items: flex-end;
  justify-content: flex-end;
  align-self: end;
  padding: 0 24px 17px;
  pointer-events: none;
}

.timeline-footer > * { pointer-events: auto; }

/* Dayflow's copyTimelineButton: the theme's secondary button, 26pt tall. */
.copy-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 122px;
  height: 26px;
  padding: 0 8px;
  overflow: hidden;
  border: 1px solid var(--dg-button-secondary-border);
  border-radius: 7px;
  background: var(--dg-button-secondary-fill);
  color: var(--dg-button-secondary-text);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: transform 300ms cubic-bezier(0.34, 1.4, 0.64, 1), background-color var(--dg-motion-fast) ease;
}

:root[data-dg-appearance='dark'] .copy-button {
  -webkit-backdrop-filter: blur(12px);
  backdrop-filter: blur(12px);
}

.copy-button:hover:not(:disabled) { transform: scale(1.02); background: var(--dg-button-secondary-hover); }
.copy-button:active:not(:disabled) { transform: scale(0.97); transition-duration: 120ms; }
.copy-button:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.copy-button:disabled { opacity: 0.5; cursor: default; }
.copy-button.is-copied { color: var(--dg-success); }
.copy-button.is-failed { color: var(--dg-danger); }

.copy-button__content {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

/* Copy state swap: slide up and fade, never scale the letterforms. */
.copy-swap-enter-active,
.copy-swap-leave-active { transition: opacity 140ms ease, transform 200ms cubic-bezier(0.165, 0.84, 0.44, 1); }
.copy-swap-enter-from { opacity: 0; transform: translateY(8px); }
.copy-swap-leave-to { opacity: 0; transform: translateY(-8px); }

/* Dayflow's CardsToReviewBadge: orange-to-lavender pill with a stacked-cards count. */
.review-badge {
  position: absolute;
  bottom: 17px;
  left: 50%;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 10px 14px 10px 12px;
  border: none;
  border-radius: 20px;
  background: linear-gradient(to bottom right, #ff9970 5%, #bdabff 95%);
  box-shadow: inset 0 0 0 1.5px #ffd9d4, 0 2px 3px #e8c9b3;
  color: #ffffff;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
  transform: translateX(-50%);
  transition: transform 300ms cubic-bezier(0.34, 1.4, 0.64, 1);
}

:root[data-dg-appearance='dark'] .review-badge { box-shadow: inset 0 0 0 1.5px #ffd9d4, 0 2px 4px rgba(0, 0, 0, 0.35); }

.review-badge:hover { transform: translateX(-50%) scale(1.02); }
.review-badge:active { transform: translateX(-50%) scale(0.97); transition-duration: 120ms; }
.review-badge:focus-visible { outline: none; box-shadow: inset 0 0 0 1.5px #ffd9d4, 0 0 0 3px var(--dg-focus-ring); }

.review-badge__stack {
  position: relative;
  flex: none;
  width: 21px;
  height: 20px;
}

.review-badge__back,
.review-badge__front {
  position: absolute;
  bottom: 0;
  border-radius: 3.5px;
  background: #ffffff;
}

.review-badge__back {
  left: 0.5px;
  width: 14px;
  height: 15.75px;
  transform: rotate(-11.64deg);
}

.review-badge__front {
  left: 7px;
  display: grid;
  width: 14px;
  height: 18px;
  box-shadow: 0 0 0 1.25px #f79c82;
  color: #fa997d;
  font-size: 9.5px;
  font-style: normal;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  place-items: center;
}

.footer-pop-enter-active,
.footer-pop-leave-active { transition: opacity 200ms ease, transform 260ms cubic-bezier(0.165, 0.84, 0.44, 1); }
.footer-pop-enter-from,
.footer-pop-leave-to { opacity: 0; transform: translateX(-50%) translateY(10px); }

@media (prefers-reduced-motion: reduce) {
  .copy-button,
  .review-badge,
  .copy-swap-enter-active,
  .copy-swap-leave-active,
  .footer-pop-enter-active,
  .footer-pop-leave-active { transition: none; }
}

/* Category manager modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(4px);
}

.modal-panel {
  width: min(480px, calc(100vw - 32px));
  max-height: calc(100vh - 64px);
  display: flex;
  flex-direction: column;
  border: 1px solid var(--dg-panel-border);
  border-radius: 12px;
  background: var(--dg-surface);
  box-shadow: var(--dg-shadow-lg);
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--dg-panel-border);
}

.modal-header h2 {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 650;
}

.modal-close {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--dg-text-secondary);
  font-size: 20px;
  cursor: pointer;
}

.modal-close:hover { background: var(--dg-hover-fill); }

.modal-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
}

.modal-empty {
  color: var(--dg-text-muted);
  font-size: 13px;
  text-align: center;
  padding: 24px 0;
}

.category-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.category-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 9px 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: var(--dg-track-fill);
  color: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  /* Fill-only hover, mirroring the filter chips: no lift, no displacement. */
  transition: background var(--dg-motion-fast) ease, border-color var(--dg-motion-fast) ease;
}

.category-item:hover { background: var(--dg-hover-fill); }
.category-item:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.category-item.is-active { border-color: var(--dg-chip-border); background: var(--dg-chip-fill); }

.category-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.category-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.category-meta {
  display: inline-flex;
  flex: none;
  align-items: baseline;
  gap: 8px;
  color: var(--dg-text-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.category-meta b {
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 20px;
  border-top: 1px solid var(--dg-panel-border);
}

.modal-hint {
  margin: 0;
  color: var(--dg-text-muted);
  font-size: 11px;
}
</style>
