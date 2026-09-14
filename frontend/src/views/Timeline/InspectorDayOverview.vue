<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, DayGoalDTO, TimelineDayDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import { useDurationFormat } from '@/lib/duration'
import { categoryLabel } from '@/lib/categoryLabel'
import type { TimelineAction } from '@/stores/timeline'

import GoalEditor from './GoalEditor.vue'
import { safeCategoryColor } from './layout'

/* The inspector's no-selection pane: day totals, per-category time, the
   day-goal form and the failed ranges that can be retried. */
const props = defineProps<{
  day: TimelineDayDTO
  canWrite: boolean
  actions: TimelineActionAvailability
  pendingAction: TimelineAction | null
  goal: DayGoalDTO | null
  goalUnavailable: boolean
  goalFailed: boolean
  goalSaving: boolean
}>()

const emit = defineEmits<{
  retry: [batchIDs: number[]]
  saveGoal: [goal: DayGoalDTO]
}>()

const { t } = useI18n()
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
  const totals = new Map<string, number>()
  for (const card of props.day.cards) {
    if (card.isIdle || card.category === 'System') continue
    totals.set(card.category, (totals.get(card.category) ?? 0) + card.durationMinutes)
  }

  const denominator = Math.max(1, [...totals.values()].reduce((sum, value) => sum + value, 0))
  return props.day.categories
    .filter((category) => totals.has(category.name))
    .map((category) => {
      const minutes = totals.get(category.name) ?? 0
      return { category, minutes, percentage: (minutes / denominator) * 100 }
    })
    .sort((left, right) => right.minutes - left.minutes)
})
</script>

<template>
  <header class="inspector__header">
    <div>
      <p class="inspector__eyebrow">{{ t('timeline.overview.eyebrow') }}</p>
      <h2 class="inspector__title dg-display">{{ t('timeline.overview.title') }}</h2>
    </div>
  </header>

  <div class="totals">
    <div class="total total--primary">
      <strong>{{ duration(props.day.trackedMinutes) }}</strong>
      <span>{{ t('timeline.overview.tracked') }}</span>
    </div>
    <div class="total">
      <strong>{{ duration(props.day.idleMinutes) }}</strong>
      <span>{{ t('timeline.overview.idle') }}</span>
    </div>
  </div>

  <div class="category-list">
    <p v-if="categoryTotals.length === 0" class="category-list__empty">
      {{ t('timeline.overview.noCategories') }}
    </p>
    <div v-for="item in categoryTotals" :key="item.category.id" class="category-total">
      <div class="category-total__meta">
        <span>
          <i :style="{ background: safeCategoryColor(item.category.colorHex) }"></i>
          {{ categoryLabel(item.category.name, t) }}
        </span>
        <strong>{{ duration(item.minutes) }}</strong>
      </div>
      <div class="category-total__track" aria-hidden="true">
        <span
          :style="{
            width: `${item.percentage}%`,
            background: safeCategoryColor(item.category.colorHex),
          }"
        ></span>
      </div>
    </div>
  </div>

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
</template>

<style scoped>
.goal-state {
  padding: 10px 12px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.goal-state span { color: var(--dg-text-secondary); font-size: 11px; }

.totals {
  display: grid;
  grid-template-columns: 1.35fr 1fr;
  gap: 8px;
  padding: 18px 0 22px;
}

.total {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 13px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.total strong {
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.total span,
.category-list__empty {
  color: var(--dg-text-muted);
  font-size: 10px;
}

.total--primary {
  border-color: color-mix(in srgb, var(--dg-accent) 22%, transparent);
  background: var(--dg-control-fill);
}

.category-list { display: grid; gap: 15px; }
.category-total__meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
  color: var(--dg-text-secondary);
  font-size: 11px;
}

.category-total__meta span { display: flex; align-items: center; gap: 7px; min-width: 0; }
.category-total__meta i { width: 7px; height: 7px; border-radius: 50%; }
.category-total__meta strong { color: var(--dg-text-primary); font-weight: 600; }
.category-total__track { height: 4px; overflow: hidden; border-radius: 99px; background: var(--dg-track-fill); }
.category-total__track span { display: block; height: 100%; border-radius: inherit; }

.inspector__failures {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding-top: 18px;
  border-top: 1px solid var(--dg-timeline-grid);
}

.inspector__retry { color: var(--dg-text-secondary); }
</style>
