<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import DevelopmentBadge from '@/components/DevelopmentBadge.vue'
import PageHeader from '@/components/PageHeader.vue'
import PeriodNav from '@/components/PeriodNav.vue'
import { calendarDayQuery, shiftCalendarDate } from '@/lib/calendarDate'
import { categoryLabel } from '@/lib/categoryLabel'
import { delayUntilDayContextRefresh } from '@/lib/dayContextRefresh'
import { formatTimelineForClipboard } from '@/lib/timelineClipboard'
import { formatTimeZoneName } from '@/lib/timeFormat'
import { safeTimeZone } from '@/lib/timeZone'
import { useTimelineStore } from '@/stores/timeline'
import TimelineInspector from './TimelineInspector.vue'
import TimelineStatePanel from './TimelineStatePanel.vue'
import TimelineTrack from './TimelineTrack.vue'
import { safeCategoryColor } from './layout'

const timeline = useTimelineStore()
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
  actionError,
  actionAvailability,
} = storeToRefs(timeline)
const { locale, t } = useI18n()
const route = useRoute()
const router = useRouter()
const copyState = ref<'idle' | 'copied' | 'failed'>('idle')
const confirmingReprocess = ref(false)
let dayRefreshTimer: number | null = null
const dateTitle = computed(() => {
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

const hasTrack = computed(() =>
  day.value !== null && ['populated', 'processing', 'failure'].includes(state.value),
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
  if (document.visibilityState === 'visible' && isFollowingToday()) void timeline.load()
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
  const succeeded = await timeline.reprocessCurrentDay(context.value.day)
  if (succeeded) confirmingReprocess.value = false
}

onMounted(() => {
  timeline.startEvents()
  window.addEventListener('focus', refreshWhenWindowReturns)
  document.addEventListener('visibilitychange', refreshWhenWindowReturns)
})
watch(() => route.query.day, () => { void timeline.load(routeDay()) }, { immediate: true })
watch([() => route.query.day, () => context.value?.dayEndTs], scheduleDayRefresh, { immediate: true })
onBeforeUnmount(() => {
  stopDayRefresh()
  window.removeEventListener('focus', refreshWhenWindowReturns)
  document.removeEventListener('visibilitychange', refreshWhenWindowReturns)
  timeline.stopListening()
})
</script>

<template>
  <div class="page timeline-page" @keydown.esc="timeline.selectCard(null)">
    <PageHeader :title="dateTitle">
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
      </template>

      <template #trail>
        <DevelopmentBadge v-if="usingDevelopmentFixture">{{ t('timeline.developmentFixture') }}</DevelopmentBadge>
        <div v-if="day" class="day-meta">
          <span>{{ t('timeline.meta.tracked', { count: day.trackedMinutes }) }}</span>
          <i aria-hidden="true"></i>
          <span v-if="localizedTimeZone">{{ localizedTimeZone }}</span>
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
        class="filter-manage filter-manage--available"
        :disabled="cards.length === 0"
        @click="copyTimeline"
      >
        {{ copyState === 'copied' ? t('timeline.copy.copied') : copyState === 'failed' ? t('timeline.copy.failed') : t('timeline.copy.action') }}
      </button>
      <button
        type="button"
        class="filter-manage"
        :title="t('timeline.filter.manageUnavailable')"
        disabled
      >
        {{ t('timeline.filter.manage') }}
      </button>
      <button
        v-if="actionAvailability.reprocessDay && (capabilities?.canWrite ?? false)"
        type="button"
        class="filter-manage"
        :disabled="pendingAction !== null || !hasTrack"
        @click="confirmingReprocess = true"
      >
        {{ t('timeline.reprocess.action') }}
      </button>
      <span v-if="confirmingReprocess" class="filter-error" role="alert">
        {{ t('timeline.reprocess.confirm') }}
        <button
          type="button"
          class="filter-manage"
          :disabled="pendingAction !== null"
          @click="reprocessCurrentDay"
        >
          {{ pendingAction === 'reprocess-day' ? t('timeline.reprocess.running') : t('timeline.reprocess.confirmYes') }}
        </button>
        <button
          type="button"
          class="filter-manage"
          :disabled="pendingAction !== null"
          @click="confirmingReprocess = false"
        >
          {{ t('common.action.cancel') }}
        </button>
      </span>
      <span v-if="actionError !== null && selectedCard === null" class="filter-error" role="alert">
        {{ t('timeline.actionFailed') }}
      </span>
    </div>

    <div class="timeline-body">
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
          @select="timeline.selectCard"
          @select-failure="timeline.selectFailure"
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
          :action-failed="actionError !== null"
          @close="timeline.selectCard(null)"
          @save-edits="(cardID, edits) => timeline.saveCardEdits(cardID, edits)"
          @delete="timeline.removeCard"
          @retry="timeline.retryFailure"
          @dismiss-failure="timeline.dismissFailure"
        />
      </template>
    </div>
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


.day-meta i {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: currentColor;
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
  gap: 7px;
  min-height: 28px;
  padding: 5px 10px;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--dg-text-secondary);
  font-size: 11px;
  white-space: nowrap;
  transition: background var(--dg-motion-fast) ease, border-color var(--dg-motion-fast) ease;
}

.filter-chip:hover { background: var(--dg-hover-fill); }
.filter-chip:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.filter-chip.is-selected { border-color: var(--dg-chip-border); background: var(--dg-chip-fill); color: var(--dg-text-primary); }
.filter-chip i { width: 7px; height: 7px; border-radius: 50%; }
.filter-bar__spacer { flex: 1; }
.filter-manage { color: var(--dg-text-muted); cursor: default; }
.filter-manage--available:not(:disabled) { color: var(--dg-text-secondary); cursor: pointer; }
.filter-manage--available:not(:disabled):hover { background: var(--dg-hover-fill); color: var(--dg-text-primary); }
.filter-manage--available:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.filter-error { flex: none; color: var(--dg-danger); font-size: 10px; white-space: nowrap; }

.timeline-body {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) var(--dg-inspector-width);
  gap: var(--dg-inspector-gap);
  flex: 1;
  min-height: 0;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

.timeline-body__state { grid-column: 1 / -1; }
.timeline-body__track,
.timeline-body__inspector { min-width: 0; }

@media (max-width: 1000px) {
  .timeline-body { grid-template-columns: minmax(0, 1fr); }
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
</style>
