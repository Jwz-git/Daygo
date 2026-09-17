<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, DayGoalDTO, TimelineDayDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import { useDurationFormat } from '@/lib/duration'
import { categoryLabel } from '@/lib/categoryLabel'
import type { TimelineAction } from '@/stores/timeline'

import GoalEditor from './GoalEditor.vue'
import type { ReviewTotals } from './review'
import { safeCategoryColor } from './layout'
import { buildDonutSectors, type DonutSector, type DonutSlice } from './donut'

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

const emit = defineEmits<{
  retry: [batchIDs: number[]]
  saveGoal: [goal: DayGoalDTO]
  reprocess: []
}>()

const { t, locale } = useI18n()
const duration = useDurationFormat()

// The retryable flag describes automatic requeue behavior only — the backend
// RetryBatches deliberately ignores both the attempt cap and failure kind
// ("not to overrule the user asking for one more run"), so every failed range
// with batches gets a manual retry button regardless of that flag.
const failuresWithBatches = computed(() =>
  props.day.failures.filter((failure) => failure.batchIds.length > 0),
)

const canRetry = computed(() => props.canWrite && props.actions.retryBatches)

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
      colorHex: '#888888',
      details: '',
      sortOrder: 999,
      isSystem: false,
      isIdle: false,
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

/*
 * The review split (你的回顾): session judgments only, one rounded block per
 * verdict on a grey track, widths proportional to minutes. Zero-state shows
 * a hint instead of an empty bar.
 */
const VERDICT_COLORS = {
  distraction: '#ef8a7a',
  neutral: '#e7e4ec',
  focus: '#35c3a2',
} as const

const reviewSegments = computed(() => [
  { label: t('timeline.review.distraction'), minutes: props.reviewTotals.distractionMinutes, color: VERDICT_COLORS.distraction },
  { label: t('timeline.review.neutral'), minutes: props.reviewTotals.neutralMinutes, color: VERDICT_COLORS.neutral },
  { label: t('timeline.review.focus'), minutes: props.reviewTotals.focusMinutes, color: VERDICT_COLORS.focus },
])

</script>

<template>
  <header class="inspector__header">
    <div>
      <p class="inspector__eyebrow">{{ t('timeline.overview.eyebrow') }}</p>
      <h2 class="inspector__title dg-display">{{ t('timeline.overview.title') }}</h2>
    </div>
  </header>

  <div class="donut" role="img" :aria-label="t('timeline.overview.donutAria')">
    <svg viewBox="0 0 205 205" aria-hidden="true">
      <defs>
        <radialGradient id="donut-sheen" cx="50%" cy="50%" r="50%">
          <stop offset="76%" stop-color="rgba(255, 255, 255, 0.35)" />
          <stop offset="100%" stop-color="rgba(255, 255, 255, 0)" />
        </radialGradient>
      </defs>
      <!-- Grey base circle with the soft ambient shadow. -->
      <circle class="donut__base" cx="102.5" cy="102.5" r="102.5" />
      <!-- Category sectors: filled annular wedges with angular gaps and
           round-cornered edges (the round-join stroke is the same colour). -->
      <g class="donut__sectors">
        <path
          v-for="(sector, index) in donutSectors"
          :key="index"
          :d="sector.path"
          :fill="sector.color"
          :stroke="sector.color"
          stroke-width="6"
          stroke-linejoin="round"
        />
      </g>
      <!-- White radial sheen from the inner edge fading outward. -->
      <circle cx="102.5" cy="102.5" r="102.5" fill="url(#donut-sheen)" />
      <!-- White center disk, slightly smaller than the hole: leaves the grey
           gap ring on the inner edge like the reference. -->
      <circle class="donut__center-disk" cx="102.5" cy="102.5" r="73" />
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
    <div v-for="slice in donutSlices" :key="slice.label" class="legend-chip">
      <span class="legend-chip__name">
        <i :style="{ background: slice.color }"></i>
        {{ slice.label }}
      </span>
      <strong>{{ duration(slice.minutes) }}</strong>
    </div>
  </div>

  <section class="inspector__section">
    <h3>{{ t('timeline.overview.review') }}</h3>
    <!-- Same verdict split as the review flow's completion bar; every block
         draws, zero-minute verdicts keep a small stub. -->
    <div class="review-bar" aria-hidden="true">
      <span
        v-for="segment in reviewSegments.filter((entry) => entry.minutes > 0)"
        :key="segment.label"
        :style="{ flexGrow: segment.minutes, '--seg': segment.color }"
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
  </section>

  <section class="inspector__section">
    <h3>{{ t('daily.goal.title') }}</h3>
    <p class="inspector__failure-note">{{ t('daily.goal.description') }}</p>
    <div v-if="goalUnavailable || goalFailed" class="goal-state">
      <span>{{ goalFailed ? t('daily.goal.failureDescription') : t('daily.goal.unavailableDescription') }}</span>
    </div>
    <GoalEditor
      v-else
      :goal="goal"
      :categories="day.categories"
      :saving="goalSaving"
      @save="(next) => emit('saveGoal', next)"
    />
  </section>

  <section v-if="failuresWithBatches.length > 0" class="inspector__section inspector__failures">
    <h3>{{ t('timeline.failure.title') }}</h3>
    <p class="inspector__failure-note">{{ t('timeline.failure.retryHint') }}</p>
    <button
      v-for="failure in failuresWithBatches"
      :key="`${failure.startTs}-${failure.endTs}`"
      type="button"
      class="dg-button inspector__retry"
      :disabled="!canRetry || props.pendingAction !== null"
      :title="canRetry ? t('timeline.failure.retry') : t('timeline.failure.retryUnavailable')"
      @click="emit('retry', failure.batchIds)"
    >
      {{ props.pendingAction === 'retry-batches' ? t('timeline.failure.retrying') : t('timeline.failure.retry') }}
    </button>
  </section>

  <section v-if="props.actions.reprocessDay" class="inspector__section">
    <button
      type="button"
      class="dg-button inspector__reprocess"
      :disabled="!props.canWrite || props.pendingAction !== null"
      :title="props.canWrite ? t('timeline.reprocess.action') : t('timeline.inspector.actionsUnavailable')"
      @click="emit('reprocess')"
    >
      <svg viewBox="0 0 16 16" aria-hidden="true" class="reprocess-icon"><path d="M13.65 2.35A8 8 0 1 0 16 8" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><path d="M11 2l3 0 0 3" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
      {{ t('timeline.reprocess.action') }}
    </button>
  </section>
</template>

<style scoped>
.goal-state {
  padding: 10px 12px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.goal-state span { color: var(--dg-text-secondary); font-size: 11px; }

/* Dayflow-spec donut: grey base circle + ambient shadow, 80%-opacity
   sectors with rounded caps, white radial sheen, and a white center disk
   that leaves a grey gap ring on the inner edge. */
.donut {
  position: relative;
  width: 205px;
  margin: 6px auto 4px;
}

.donut svg {
  display: block;
  width: 100%;
  /* The base circle fills the viewBox exactly, so its drop-shadow lives
     outside the viewport — let it render instead of clipping. */
  overflow: visible;
}

.donut__base {
  fill: #f1f1f4;
  filter: drop-shadow(0 0 10px rgba(45, 50, 80, 0.18));
}

:root[data-dg-appearance='dark'] .donut__base { fill: #26262c; }

.donut__sectors path {
  fill-opacity: 0.8;
}

.donut__center-disk { fill: var(--dg-surface, #ffffff); }

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
  color: #b1b1b1;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.donut__center strong {
  color: var(--dg-text-primary);
  font-family: var(--dg-font-reading);
  font-size: 17px;
  font-weight: 450;
  line-height: 1.25;
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
}

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
.review-bar {
  display: flex;
  gap: 8px;
  min-height: 44px;
  margin-top: 10px;
  padding: 5px;
  border-radius: 12px;
  background: var(--dg-track-fill);
}

.review-bar span {
  flex-basis: 40px;
  border-radius: 10px;
  /* Cylindrical sheen: lighter crown, darker base, soft drop shadow. */
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--seg) 72%, #ffffff),
    var(--seg) 55%,
    color-mix(in srgb, var(--seg) 86%, #000000)
  );
  box-shadow: 0 6px 14px rgba(45, 50, 80, 0.16);
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
  padding-top: 18px;
  border-top: 1px solid var(--dg-timeline-grid);
}

.inspector__retry { color: var(--dg-text-secondary); }

.inspector__reprocess {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 10px 14px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 8px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.inspector__reprocess:hover:not(:disabled) {
  border-color: var(--dg-accent);
  color: var(--dg-accent);
  background: var(--dg-accent-subtle);
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
