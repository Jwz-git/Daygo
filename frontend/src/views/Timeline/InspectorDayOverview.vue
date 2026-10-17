<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, DayGoalDTO, TimelineDayDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import { useDurationFormat } from '@/lib/duration'
import { categoryLabel } from '@/lib/categoryLabel'
import type { TimelineAction } from '@/stores/timeline'

import GoalEditor from './GoalEditor.vue'
import GoalProgress from './GoalProgress.vue'
import PlanPanel from './PlanPanel.vue'
import type { ReviewTotals } from './review'
import { FALLBACK_CATEGORY_COLOR, safeCategoryColor } from './layout'
import { failurePresentation } from './failurePresentation'
import { buildDonutSectors, fullRingPath, type DonutSector, type DonutSlice } from './donut'

/*
 * The inspector's no-selection pane, laid out like the Dayflow reference:
 * a "today so far" donut with a category legend, the review-verdict split,
 * then the day-goal form and the retryable failures.
 */
const props = defineProps<{
  day: TimelineDayDTO
  canWrite: boolean
  actions: TimelineActionAvailability
  pendingAction: TimelineAction | null
  goal: DayGoalDTO | null
  goalUnavailable: boolean
  goalFailed: boolean
  goalSaving: boolean
  reviewTotals: ReviewTotals
}>()

// The displayed day is the live one when now falls in its backend window.
const isLiveDay = computed(() => {
  const now = Math.floor(Date.now() / 1000)
  return props.day.dayStartTs <= now && now < props.day.dayEndTs
})

const emit = defineEmits<{
  retry: [batchIDs: number[]]
  stopRetries: [batchIDs: number[]]
  saveGoal: [goal: DayGoalDTO]
  reprocess: []
}>()

const { t, locale } = useI18n()

/*
 * The day goal stays collapsed to its progress bars; the editor opens only on
 * "set goals" / edit and folds away again once a save lands.
 */
const editingGoal = ref(false)
const goalActive = computed(() => {
  const goal = props.goal
  return goal !== null && goal.exists && !goal.isSkipped
    && (goal.focusTargetMinutes > 0 || goal.distractionLimitMinutes > 0)
})
watch(() => props.goalSaving, (saving, wasSaving) => {
  if (wasSaving && !saving && !props.goalFailed) editingGoal.value = false
})
watch(() => props.day.day, () => { editingGoal.value = false })
const duration = useDurationFormat()

// The retryable flag describes automatic requeue behavior only — the backend
// RetryBatches deliberately ignores both the attempt cap and failure kind
// ("not to overrule the user asking for one more run"), so every failed range
// with batches gets a manual retry button regardless of that flag.
const failuresWithBatches = computed(() =>
  props.day.failures.filter((failure) => failure.batchIds.length > 0),
)
const providerFailureCount = computed(() =>
  failuresWithBatches.value.filter((failure) => failurePresentation(failure.kind).source === 'provider').length,
)

// A single retry submits every failed range's batches at once. The backend
// requeues them to pending and the scheduler works through them one tick at a
// time (respecting rate-limit pacing), so this is a sequential retry of all
// failures behind one button rather than a button per range.
const allFailedBatchIds = computed(() =>
  failuresWithBatches.value.flatMap((failure) => failure.batchIds),
)

// "Stop all" only targets the failures still on the auto-retry track — the
// same predicate the failure detail pane uses to decide whether stopping does
// anything. Batches whose kind needs attention (or already stopped) are left
// out so the button hides once nothing is auto-retrying.
const retryableFailedBatchIds = computed(() =>
  failuresWithBatches.value
    .filter((failure) => failure.retryable)
    .flatMap((failure) => failure.batchIds),
)

const canRetry = computed(() => props.canWrite && props.actions.retryBatches)
const canStopAll = computed(
  () => props.canWrite && props.actions.stopRetries && retryableFailedBatchIds.value.length > 0,
)

interface CategoryTotal {
  category: CategoryDTO
  minutes: number
  percentage: number
}

const categoryTotals = computed<CategoryTotal[]>(() => {
  const categoryMap = new Map<string, CategoryDTO>()
  for (const cat of props.day.categories) {
    categoryMap.set(cat.name, cat)
  }

  const totals = new Map<string, number>()
  for (const card of props.day.cards) {
    if (card.isIdle || card.category === 'System') continue
    totals.set(card.category, (totals.get(card.category) ?? 0) + card.durationMinutes)
  }

  const denominator = Math.max(1, [...totals.values()].reduce((sum, value) => sum + value, 0))
  const result: CategoryTotal[] = []
  for (const [name, minutes] of totals.entries()) {
    const category = categoryMap.get(name) ?? {
      id: '0',
      name,
      colorHex: FALLBACK_CATEGORY_COLOR,
      details: '',
      sortOrder: 999,
      isSystem: false,
      isIdle: false,
      archetype: 'forge',
      createdAtTs: 0,
      updatedAtTs: 0,
    }
    result.push({ category, minutes, percentage: (minutes / denominator) * 100 })
  }
  return result.sort((left, right) => right.minutes - left.minutes)
})

/*
 * Ring chart of the analyzed day: one slice per category plus the idle cards
 * in grey. The center total is the overall recorded time the pipeline has
 * accounted for; unanalyzed wall-clock is intentionally absent (Dayflow-style).
 */
const IDLE_COLOR = '#c9c6d2'

const donutSlices = computed<DonutSlice[]>(() => {
  const slices: DonutSlice[] = categoryTotals.value.map((item) => ({
    label: categoryLabel(item.category.name, t),
    minutes: item.minutes,
    color: safeCategoryColor(item.category.colorHex),
  }))
  if (props.day.idleMinutes > 0) {
    slices.push({ label: t('timeline.overview.idle'), minutes: props.day.idleMinutes, color: IDLE_COLOR })
  }
  return slices
})

const centerMinutes = computed(() =>
  donutSlices.value.reduce((sum, slice) => sum + slice.minutes, 0),
)

/* Hours on one line, minutes on the next; Chinese duration strings carry
   wide full-width spaces, and the donut reads tighter without them. */
const centerLines = computed(() => {
  const total = Math.max(0, Math.round(centerMinutes.value))
  const hours = Math.floor(total / 60)
  const minutes = total % 60
  const lines: string[] = []
  if (hours > 0) lines.push(t('common.duration.hours', { count: hours }))
  lines.push(t('common.duration.minutes', { count: minutes }))
  const compact = (line: string): string =>
    locale.value.startsWith('zh') ? line.replace(/\s+/g, '') : line
  return lines.map(compact)
})

const donutSectors = computed<DonutSector[]>(() =>
  buildDonutSectors(donutSlices.value),
)

// The ring band (outer 102.5, inner 0.75 of it, as the sectors) that the glass
// sheen paints over, so the light never spills into the hole.
const GLASS_RING = fullRingPath(102.5, 102.5, 102.5, 76.875)


// Sectors and legend chips share the donutSlices order 1:1 (every slice has
// positive minutes), so a single hovered index links the two: the matching
// wedge lifts to full strength while the rest recede.
const activeSlice = ref<number | null>(null)

/*
 * The review split (你的回顾): session judgments only, one rounded block per
 * verdict on a grey track, widths proportional to minutes. Zero-state shows
 * a hint instead of an empty bar.
 */
const VERDICT_COLORS = {
  distraction: 'var(--dg-danger)',
  // Mid grey: the old #e7e4ec vanished into the meter's light track.
  neutral: '#bcb8c4',
  focus: '#35c3a2',
} as const

const reviewSegments = computed(() => [
  { label: t('timeline.review.distraction'), minutes: props.reviewTotals.distractionMinutes, color: VERDICT_COLORS.distraction },
  { label: t('timeline.review.neutral'), minutes: props.reviewTotals.neutralMinutes, color: VERDICT_COLORS.neutral },
  { label: t('timeline.review.focus'), minutes: props.reviewTotals.focusMinutes, color: VERDICT_COLORS.focus },
])

const reviewMinutesTotal = computed(() =>
  reviewSegments.value.reduce((sum, segment) => sum + segment.minutes, 0),
)

</script>

<template>
  <header class="inspector__header">
    <div class="inspector__heading">
      <p class="inspector__eyebrow">{{ t('timeline.overview.eyebrow') }}</p>
      <h2 class="inspector__title inspector__title--card">{{ t('timeline.overview.title') }}</h2>
    </div>
  </header>

  <div
    class="donut"
    :class="{ 'donut--focused': activeSlice !== null }"
    role="img"
    :aria-label="t('timeline.overview.donutAria')"
  >
    <svg viewBox="0 0 205 205" aria-hidden="true">
      <defs>
        <!-- Shadows are SVG filters, not CSS filter: WebKit (the macOS
             WKWebView) does not reliably paint CSS filters on SVG children,
             so drop-shadow() on these elements rendered nothing. -->
        <!-- Ambient shadow under the grey base. -->
        <filter id="donut-base-shadow" x="-30%" y="-30%" width="160%" height="170%">
          <feDropShadow class="donut-flood--base" dx="0" dy="6" stdDeviation="8" />
        </filter>
        <!-- Each wedge casts a shadow in its own colour, like the meter
             capsules: the blurred, offset wedges themselves, faded. -->
        <filter id="donut-sector-shadow" x="-20%" y="-20%" width="140%" height="145%">
          <feGaussianBlur in="SourceGraphic" stdDeviation="2.2" result="blur" />
          <feOffset in="blur" dy="2" result="offset" />
          <feComponentTransfer in="offset" result="shadow">
            <feFuncA type="linear" slope="0.6" />
          </feComponentTransfer>
          <feMerge>
            <feMergeNode in="shadow" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
        <!-- Lift for the white center disk. -->
        <filter id="donut-disk-shadow" x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow class="donut-flood--disk" dx="0" dy="1" stdDeviation="2" />
        </filter>
        <!-- Glass light: a sheen pooled at the upper left fading out across
             the ring, and a rim light bright on top, gone at the bottom. -->
        <linearGradient id="donut-glass-sheen" x1="0.15" y1="0" x2="0.75" y2="1">
          <stop class="donut-glass--in" offset="0%" />
          <stop class="donut-glass--mid" offset="38%" />
          <stop class="donut-glass--out" offset="72%" />
        </linearGradient>
        <linearGradient id="donut-glass-rim" x1="0" y1="0" x2="0" y2="1">
          <stop class="donut-rim--top" offset="0%" />
          <stop class="donut-rim--bottom" offset="40%" />
        </linearGradient>
        <!-- Soft contact shadow hugging the inner edge of the ring. -->
        <radialGradient id="donut-inner-shade" cx="50%" cy="50%" r="50%">
          <stop offset="70%" stop-color="rgba(20, 22, 40, 0)" />
          <stop class="donut-stop--rim" offset="75%" />
          <stop offset="80%" stop-color="rgba(20, 22, 40, 0)" />
        </radialGradient>
      </defs>
      <!-- Grey base circle with the soft ambient shadow. -->
      <circle class="donut__base" cx="102.5" cy="102.5" r="102.5" filter="url(#donut-base-shadow)" />
      <!-- Category sectors: filled annular wedges with angular gaps and
           round-cornered edges (the round-join stroke is the same colour). -->
      <g class="donut__sectors" filter="url(#donut-sector-shadow)">
        <path
          v-for="(sector, index) in donutSectors"
          :key="index"
          class="donut__sector"
          :class="{
            'is-active': activeSlice === index,
            'is-muted': activeSlice !== null && activeSlice !== index,
          }"
          :d="sector.path"
          :fill="sector.color"
          :stroke="sector.color"
          stroke-width="6"
          stroke-linejoin="round"
          @mouseenter="activeSlice = index"
          @mouseleave="activeSlice = null"
        />
      </g>
      <!-- Glass light over the ring band: sheen, then the outer rim light. -->
      <path class="donut__glass" :d="GLASS_RING" fill="url(#donut-glass-sheen)" />
      <circle class="donut__rim" cx="102.5" cy="102.5" r="101.6" fill="none" stroke="url(#donut-glass-rim)" stroke-width="0.8" />
      <!-- Contact shadow at the hole, then the raised white center disk. -->
      <circle cx="102.5" cy="102.5" r="102.5" fill="url(#donut-inner-shade)" />
      <circle class="donut__center-disk" cx="102.5" cy="102.5" r="73" filter="url(#donut-disk-shadow)" />
    </svg>
    <div class="donut__center">
      <span class="donut__total-label">{{ t('timeline.overview.total') }}</span>
      <strong v-for="line in centerLines" :key="line">{{ line }}</strong>
    </div>
  </div>

  <div class="donut-legend">
    <p v-if="donutSlices.length === 0" class="donut-legend__empty">
      {{ t('timeline.overview.noCategories') }}
    </p>
    <div
      v-for="(slice, index) in donutSlices"
      :key="slice.label"
      class="legend-chip"
      :class="{
        'is-active': activeSlice === index,
        'is-muted': activeSlice !== null && activeSlice !== index,
      }"
      @mouseenter="activeSlice = index"
      @mouseleave="activeSlice = null"
    >
      <span class="legend-chip__name">
        <i :style="{ background: slice.color }"></i>
        {{ slice.label }}
      </span>
      <strong>{{ duration(slice.minutes) }}</strong>
    </div>
  </div>

  <PlanPanel :day="day.day" :categories="day.categories" :can-write="canWrite" :live="isLiveDay" />

  <section class="inspector__tile goal-tile">
    <header class="inspector__tile-head">
      <h3 class="inspector__tile-title">{{ t('daily.goal.title') }}</h3>
      <template v-if="!goalUnavailable && !goalFailed && canWrite">
        <button v-if="editingGoal" type="button" class="inspector__tile-action goal-tile__collapse" @click="editingGoal = false">
          {{ t('daily.goal.progress.collapse') }}
        </button>
        <button
          v-else-if="goalActive"
          type="button"
          class="goal-tile__edit"
          :title="t('daily.goal.progress.edit')"
          :aria-label="t('daily.goal.progress.edit')"
          @click="editingGoal = true"
        >
          <DgIcon name="pencil" :size="11" />
        </button>
        <button v-else type="button" class="inspector__tile-action" @click="editingGoal = true">
          {{ t('daily.goal.progress.setGoals') }}
        </button>
      </template>
    </header>
    <p v-if="goalUnavailable || goalFailed" class="inspector__tile-note">
      {{ goalFailed ? t('daily.goal.failureDescription') : t('daily.goal.unavailableDescription') }}
    </p>
    <template v-else>
      <p v-if="!editingGoal" class="inspector__tile-note goal-tile__hint">
        {{ goal?.isSkipped ? t('daily.goal.progress.skippedHint') : goalActive ? t('daily.goal.progress.trackingHint') : t('daily.goal.progress.inactiveHint') }}
      </p>
      <Transition name="goal-swap" mode="out-in">
        <GoalEditor
          v-if="editingGoal"
          key="editor"
          :goal="goal"
          :categories="day.categories"
          :saving="goalSaving"
          @save="(next) => emit('saveGoal', next)"
        />
        <GoalProgress
          v-else
          key="progress"
          :goal="goal"
          :categories="day.categories"
          :cards="day.cards"
          :active="goalActive"
        />
      </Transition>
    </template>
  </section>

  <section class="inspector__tile">
    <header class="inspector__tile-head">
      <h3 class="inspector__tile-title">{{ t('timeline.overview.review') }}</h3>
      <span v-if="reviewMinutesTotal > 0" class="inspector__tile-meta">{{ duration(reviewMinutesTotal) }}</span>
    </header>
    <!-- No judged minutes yet: a hint instead of an empty bar and three zeros,
         since review verdicts are session-local and start unset. -->
    <p v-if="reviewMinutesTotal === 0" class="inspector__tile-note">
      {{ t('timeline.overview.reviewEmpty') }}
    </p>
    <template v-else>
      <!-- Same verdict split as the review flow's completion bar; only judged
           verdicts draw a block, widths proportional to minutes. -->
      <div class="review-bar dg-meter" aria-hidden="true">
        <span
          v-for="segment in reviewSegments.filter((entry) => entry.minutes > 0)"
          :key="segment.label"
          class="dg-meter__fill"
          :style="{ flexGrow: segment.minutes, '--meter-fill': segment.color }"
        ></span>
      </div>
      <div class="review-stats">
        <div v-for="segment in reviewSegments" :key="segment.label" class="review-stat">
          <span class="review-stat__label">
            <i :style="{ background: segment.color }"></i>{{ segment.label }}
          </span>
          <strong>{{ duration(segment.minutes) }}</strong>
        </div>
      </div>
    </template>
  </section>

  <section v-if="failuresWithBatches.length > 0" class="inspector__tile inspector__failures">
    <header class="inspector__tile-head">
      <h3 class="inspector__tile-title">{{ t('timeline.failure.title') }}</h3>
    </header>
    <p v-if="providerFailureCount > 0" class="inspector__failure-note" role="status">
      {{ t('timeline.failure.providerSummary', { count: providerFailureCount }) }}
      <RouterLink :to="{ name: 'settings', query: { section: 'providers' } }">{{ t('timeline.failure.openProviders') }}</RouterLink>
    </p>
    <p class="inspector__failure-note">{{ t('timeline.failure.retryHint') }}</p>
    <button
      type="button"
      class="dg-button inspector__retry"
      :disabled="!canRetry || props.pendingAction !== null"
      :title="canRetry ? t('timeline.failure.retryAll', { count: failuresWithBatches.length }) : t('timeline.failure.retryUnavailable')"
      @click="emit('retry', allFailedBatchIds)"
    >
      {{ props.pendingAction === 'retry-batches' ? t('timeline.failure.retrying') : t('timeline.failure.retryAll', { count: failuresWithBatches.length }) }}
    </button>
    <button
      v-if="canStopAll"
      type="button"
      class="dg-button inspector__retry"
      :disabled="props.pendingAction !== null"
      :title="t('timeline.failure.stopAll')"
      @click="emit('stopRetries', retryableFailedBatchIds)"
    >
      {{ props.pendingAction === 'stop-retries' ? t('timeline.failure.stopping') : t('timeline.failure.stopAll') }}
    </button>
  </section>

  <section v-if="props.actions.reprocessDay" class="overview-reprocess">
    <button
      type="button"
      class="dg-button inspector__reprocess"
      :disabled="!props.canWrite || props.pendingAction !== null"
      :title="props.canWrite ? t('timeline.reprocess.action') : t('timeline.inspector.actionsUnavailable')"
      @click="emit('reprocess')"
    >
      <DgIcon class="reprocess-icon" name="refresh" :size="14" />
      {{ t('timeline.reprocess.action') }}
    </button>
  </section>
</template>

<style scoped>
/* Match the selected-card header: the eyebrow uses the card pane's larger type
   scale so the inspector reads as one component across its panes. The title
   shares .inspector__title--card from the shell. */
.inspector__heading .inspector__eyebrow { font-size: 13px; }


/* Dayflow-lineage donut in the meter language: a grey base with ambient
   shadow, flat wedges each casting a shadow in its own colour, a contact
   shadow at the hole and a raised white center disk. Hovering a
   wedge or its legend chip lifts that slice and lets the others recede. */
.donut {
  position: relative;
  width: 205px;
  /* Clear the header's divider: the ring's ambient shadow reaches ~16px up. */
  margin: 26px auto 4px;
}

.donut svg {
  display: block;
  width: 100%;
  /* The base circle fills the viewBox exactly, so its drop-shadow lives
     outside the viewport — let it render instead of clipping. */
  overflow: visible;
  animation: donut-rise var(--dg-motion-slow) var(--dg-ease-glide) both;
}

.donut__base { fill: #ececf1; }
:root[data-dg-appearance='dark'] .donut__base { fill: #24242a; }

.donut-flood--base { flood-color: rgb(45, 50, 80); flood-opacity: 0.2; }
.donut-flood--disk { flood-color: rgb(45, 50, 80); flood-opacity: 0.16; }
:root[data-dg-appearance='dark'] .donut-flood--base { flood-color: #000000; flood-opacity: 0.5; }
:root[data-dg-appearance='dark'] .donut-flood--disk { flood-color: #000000; flood-opacity: 0.45; }

.donut__sector {
  /* Slightly translucent so the base glows through, glass-like; the group's
     SVG filter adds a shadow in each wedge's hue. */
  fill-opacity: 0.88;
  /* Pin the hover scale to the ring centre. Without an explicit box WebKit
     pivots around each wedge's own bounding box, so the active sector drifts
     off the ring instead of lifting in place. */
  transform-box: view-box;
  transform-origin: 102.5px 102.5px;
  transition:
    fill-opacity var(--dg-motion-base) var(--dg-ease-out),
    opacity var(--dg-motion-base) var(--dg-ease-out),
    transform var(--dg-motion-base) var(--dg-ease-glide);
}

.donut__sector.is-active {
  fill-opacity: 1;
  transform: scale(1.035);
}

/* Hovering a type isolates it: the other wedges (fill + round-join stroke)
   fade out entirely, leaving only the hovered category on the grey base. */
.donut__sector.is-muted {
  opacity: 0;
}

.donut-stop--rim { stop-color: rgba(20, 22, 40, 0.16); }

:root[data-dg-appearance='dark'] .donut-stop--rim { stop-color: rgba(0, 0, 0, 0.34); }

/* Glass light: screen only brightens, so every hue keeps its colour. Neither
   layer may take the pointer from the wedges beneath. */
.donut__glass,
.donut__rim {
  pointer-events: none;
}

.donut__glass { mix-blend-mode: screen; }

.donut-glass--in { stop-color: #ffffff; stop-opacity: 0.42; }
.donut-glass--mid { stop-color: #ffffff; stop-opacity: 0.12; }
.donut-glass--out { stop-color: #ffffff; stop-opacity: 0; }
.donut-rim--top { stop-color: #ffffff; stop-opacity: 0.38; }
.donut-rim--bottom { stop-color: #ffffff; stop-opacity: 0; }

:root[data-dg-appearance='dark'] .donut-glass--in { stop-opacity: 0.22; }
:root[data-dg-appearance='dark'] .donut-glass--mid { stop-opacity: 0.06; }
:root[data-dg-appearance='dark'] .donut-rim--top { stop-opacity: 0.18; }

.donut__center-disk {
  fill: var(--dg-surface, #ffffff);
}

@keyframes donut-rise {
  from {
    opacity: 0;
    transform: scale(0.94) rotate(-6deg);
  }
  to {
    opacity: 1;
    transform: scale(1) rotate(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .donut svg { animation: none; }
  .donut__sector { transition: opacity var(--dg-motion-base) var(--dg-ease-out); }
  .donut__sector.is-active { transform: none; }
}

.donut__center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  text-align: center;
}

.donut__total-label {
  color: var(--dg-text-muted);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.donut__center strong {
  color: var(--dg-text-primary);
  font-family: var(--dg-font-reading);
  font-size: 18px;
  font-weight: 450;
  line-height: 1.22;
  font-variant-numeric: tabular-nums;
}

/* Two-line legend chips: swatch + name, duration beneath. */
.donut-legend {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px 22px;
  padding: 10px 0 4px;
}

.donut-legend__empty {
  margin: 0;
  color: var(--dg-text-muted);
  font-size: 11px;
}

.legend-chip {
  display: grid;
  justify-items: center;
  gap: 2px;
  min-width: 84px;
  padding: 3px 8px;
  border-radius: 9px;
  cursor: default;
  transition:
    background var(--dg-motion-base) var(--dg-ease-out),
    opacity var(--dg-motion-base) var(--dg-ease-out);
}

.legend-chip.is-active { background: var(--dg-hover-fill); }

.legend-chip.is-muted { opacity: 0.4; }

.legend-chip__name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  overflow: hidden;
  color: var(--dg-text-secondary);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.legend-chip__name i {
  flex: none;
  width: 12px;
  height: 9px;
  border-radius: 3px;
}

.legend-chip strong {
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 650;
}

/* Review verdict split on a grey track. */
/* The verdict split on the shared meter: one capsule per judged verdict,
   widths proportional to minutes. */
.review-bar .dg-meter__fill {
  flex: 1 1 0;
  min-width: 8px;
}

.review-stats {
  display: flex;
  justify-content: space-around;
  gap: 10px;
  margin-top: 12px;
}

.review-stat {
  display: grid;
  justify-items: center;
  gap: 3px;
}

.review-stat__label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-text-secondary);
  font-size: 11px;
}

.review-stat__label i {
  width: 16px;
  height: 10px;
  border-radius: 4px;
}

.review-stat strong {
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 650;
}

.inspector__failures {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}

.inspector__failures .inspector__tile-head { margin-bottom: 0; }

.goal-tile .goal-tile__hint { margin: -4px 0 14px; font-size: 11.5px; }

.goal-tile__edit {
  display: grid;
  width: 22px;
  height: 22px;
  margin-left: auto;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: var(--dg-hover-fill);
  color: var(--dg-text-secondary);
  cursor: pointer;
  place-items: center;
  transition: background-color var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.goal-tile__edit:hover { background: var(--dg-hover-fill-strong); color: var(--dg-text-primary); }
.goal-tile__edit:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.inspector__tile-action.goal-tile__collapse {
  background: var(--dg-hover-fill);
  color: var(--dg-text-secondary);
}

/* Progress ⇄ editor: a short fade with a slight drop, so expanding reads as
   the tile opening rather than a jump cut. */
.goal-swap-enter-active,
.goal-swap-leave-active {
  transition: opacity 160ms ease, transform 220ms var(--dg-ease-glide);
}

.goal-swap-enter-from,
.goal-swap-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media (prefers-reduced-motion: reduce) {
  .goal-swap-enter-active,
  .goal-swap-leave-active { transition: none; }
}

.overview-reprocess { margin-top: 12px; padding-bottom: 4px; }

.inspector__retry { color: var(--dg-text-secondary); }

.inspector__reprocess {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  padding: 10px 14px;
  border: 1px dashed var(--dg-timeline-grid-strong);
  border-radius: 12px;
  background: transparent;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.inspector__reprocess:hover:not(:disabled) {
  border-color: var(--dg-accent);
  color: var(--dg-accent);
  background: color-mix(in srgb, var(--dg-accent) 8%, transparent);
}

.inspector__reprocess:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.reprocess-icon {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}
</style>
