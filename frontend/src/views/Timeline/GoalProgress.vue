<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, DayGoalDTO, TimelineCardDTO } from '@/api/dto'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'

import GoalIcon from './GoalIcon.vue'
import { compactHours, goalProgress } from './goalProgress'
import { safeCategoryColor } from './layout'

/*
 * The day-goal progress bars (Dayflow DayGoalHeader): focus fills one segment
 * per focus category toward the target and glows once fulfilled; the
 * distraction budget is a bar that shrinks from the left as distraction is
 * spent. Without an active goal both tracks show greyed out, as in Dayflow's
 * disabled state. Values come from the day's cards via goalProgress().
 */
const props = defineProps<{
  goal: DayGoalDTO | null
  categories: CategoryDTO[]
  cards: TimelineCardDTO[]
  /** False shows the inactive tracks (no goal, or the day is skipped). */
  active: boolean
}>()

const { t, locale } = useI18n()
const duration = useDurationFormat()

const progress = computed(() =>
  props.goal === null
    ? { focusMinutes: 0, focusSegments: [], distractionMinutes: 0 }
    : goalProgress(props.goal, props.categories, props.cards),
)

const target = computed(() => props.goal?.focusTargetMinutes ?? 0)
const limit = computed(() => props.goal?.distractionLimitMinutes ?? 0)
const fulfilled = computed(() => target.value > 0 && progress.value.focusMinutes >= target.value)
const overBudget = computed(() => limit.value > 0 && progress.value.distractionMinutes > limit.value)

/* Chinese duration strings carry spaces that read loose in a tight row. */
function compact(minutes: number): string {
  const text = duration(minutes)
  return locale.value.startsWith('zh') ? text.replace(/\s+/g, '') : text
}

const SEGMENT_GAP = 2.5

const segments = computed(() => {
  const visible = progress.value.focusSegments.filter((segment) => segment.minutes > 0)
  const denominator = Math.max(target.value, progress.value.focusMinutes, 1)
  const gaps = SEGMENT_GAP * Math.max(visible.length - 1, 0)
  return visible.map((segment) => ({
    ...segment,
    color: safeCategoryColor(segment.colorHex),
    width: `calc((100% - ${gaps}px) * ${segment.minutes / denominator})`,
  }))
})

const legend = computed(() =>
  progress.value.focusSegments.map((segment) => ({
    id: segment.id,
    label: categoryLabel(segment.name, t),
    color: safeCategoryColor(segment.colorHex),
  })),
)

/** Share of the budget still left, 0–1: the distraction fill's width. */
const budgetLeft = computed(() =>
  limit.value <= 0 ? 0 : Math.min(1, Math.max(0, 1 - progress.value.distractionMinutes / limit.value)),
)
</script>

<template>
  <div class="goal-progress" :class="{ 'is-inactive': !active }">
    <!-- Focus: bubble on the left, the bar filling toward the target. -->
    <div class="goal-row goal-row--focus" :class="{ 'is-fulfilled': active && fulfilled }">
      <span class="goal-bubble goal-bubble--focus" aria-hidden="true">
        <GoalIcon kind="focus" :size="18" />
      </span>
      <div class="goal-row__main">
        <p class="goal-row__labels">
          <span class="goal-row__caption">{{ t('daily.goal.progress.focus') }}</span>
          <span v-if="active && target > 0" class="goal-row__value">
            <strong>{{ compactHours(progress.focusMinutes) }}</strong>{{ t('daily.goal.progress.hoursOf', { target: compactHours(target) }) }}
          </span>
          <span v-else-if="active" class="goal-row__value">{{ t('daily.goal.progress.noTarget') }}</span>
        </p>
        <div class="goal-track">
          <div v-if="active" class="goal-track__segments">
            <span
              v-for="segment in segments"
              :key="segment.id"
              class="goal-track__segment"
              :style="{ width: segment.width, '--segment': segment.color }"
            ></span>
          </div>
          <span v-else class="goal-track__idle"></span>
        </div>
        <div v-if="active && legend.length > 0" class="goal-legend">
          <span v-for="item in legend" :key="item.id"><i :style="{ background: item.color }"></i>{{ item.label }}</span>
        </div>
      </div>
    </div>

    <!-- Distraction budget: mirrored, bubble on the right; the fill is what is
         left of the budget, so it shrinks from the left as distraction lands. -->
    <div class="goal-row goal-row--distraction" :class="{ 'is-over': active && overBudget }">
      <div class="goal-row__main">
        <p class="goal-row__labels">
          <span v-if="active && limit > 0" class="goal-row__value">
            <template v-if="overBudget">
              <strong>{{ compact(progress.distractionMinutes) }}</strong>{{ t('daily.goal.progress.usedOf', { limit: compact(limit) }) }}
            </template>
            <template v-else>
              <strong>{{ compact(limit - progress.distractionMinutes) }}</strong>{{ t('daily.goal.progress.leftOf', { limit: compact(limit) }) }}
            </template>
          </span>
          <span v-else-if="active" class="goal-row__value">{{ t('daily.goal.progress.noLimit') }}</span>
          <span class="goal-row__caption">{{ t('daily.goal.progress.budget') }}</span>
        </p>
        <div class="goal-track">
          <span
            v-if="active"
            class="goal-track__budget"
            :style="{ width: `calc((100% - 6px) * ${budgetLeft})` }"
          ></span>
          <span v-else class="goal-track__idle goal-track__idle--right"></span>
        </div>
      </div>
      <span class="goal-bubble goal-bubble--distraction" aria-hidden="true">
        <GoalIcon kind="distraction" :size="19" />
      </span>
    </div>
  </div>
</template>

<style scoped>
.goal-progress {
  display: grid;
  gap: 14px;
}

.goal-row {
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
}

.goal-row--distraction { grid-template-columns: minmax(0, 1fr) 32px; }

.goal-row__main {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.goal-row__labels {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  margin: 0;
  white-space: nowrap;
}

.goal-row__caption {
  color: var(--dg-text-muted);
  font-size: 11px;
}

.goal-row__value {
  overflow: hidden;
  color: var(--dg-text-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
}

.goal-row__value strong {
  margin-right: 2px;
  font-size: 12px;
  font-weight: 650;
}

.goal-row--focus .goal-row__value strong { color: var(--dg-goal-focus-text); }
.goal-row--distraction .goal-row__value strong { color: var(--dg-goal-distraction-text); }

/* Past the budget: the used figure turns into a bold gradient, as in Dayflow. */
.goal-row--distraction.is-over .goal-row__value strong {
  background: linear-gradient(135deg, #ff8c85, #fc675f);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  font-size: 14px;
  font-weight: 750;
}

.goal-row--focus.is-fulfilled .goal-row__value strong {
  background: linear-gradient(135deg, #5b87ff, #003ee9);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  font-size: 14px;
  font-weight: 750;
}

/* ---- Tracks ---- */
.goal-track {
  position: relative;
  height: 14px;
  border-radius: 3px;
  background: var(--dg-hover-fill);
  box-shadow: inset 0 0 0 0.5px var(--dg-goal-box-border);
  transition: box-shadow var(--dg-motion-base) ease;
}

.goal-row--focus.is-fulfilled .goal-track {
  box-shadow: inset 0 0 0 0.5px rgba(145, 174, 255, 0.9), 0 0 6px rgba(98, 140, 255, 0.5);
}

.goal-track__segments {
  position: absolute;
  inset: 3px 3px 3px 3px;
  display: flex;
  gap: 2.5px;
}

.goal-track__segment {
  flex: none;
  height: 8px;
  border-radius: 999px;
  background: var(--segment);
  transition: width 820ms cubic-bezier(0.18, 0.88, 0.2, 1);
}

.goal-row--focus.is-fulfilled .goal-track__segment {
  box-shadow: 0 0 8px color-mix(in srgb, var(--segment) 30%, transparent);
}

.goal-track__budget {
  position: absolute;
  top: 4px;
  right: 3px;
  height: 6px;
  border-radius: 6px;
  background: var(--dg-goal-distraction);
  transition: width 420ms cubic-bezier(0.16, 1, 0.3, 1);
}

/* Inactive: a grey capsule resting in a grey track. */
.goal-track__idle {
  position: absolute;
  top: 4px;
  left: 3px;
  width: 92%;
  height: 6px;
  border-radius: 999px;
  background: var(--dg-hover-fill-strong);
}

.goal-track__idle--right { right: 3px; left: auto; }

/* ---- Legend ---- */
.goal-legend {
  display: flex;
  gap: 8px;
  overflow: hidden;
  padding-left: 2px;
  white-space: nowrap;
}

.goal-legend span {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 3px;
  color: var(--dg-text-secondary);
  font-size: 10px;
  font-weight: 550;
}

.goal-legend i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
}

/* ---- Bubbles ---- */
.goal-bubble {
  display: grid;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--dg-goal-box-fill);
  box-shadow: inset 0 0 0 1px var(--dg-goal-box-border), 0 1px 3px rgba(40, 30, 24, 0.06);
  place-items: center;
}

.goal-bubble--focus { color: var(--dg-goal-focus-text); }
.goal-bubble--distraction { color: var(--dg-goal-distraction-text); }

.goal-progress.is-inactive .goal-bubble { color: var(--dg-text-muted); }

@media (prefers-reduced-motion: reduce) {
  .goal-track__segment,
  .goal-track__budget { transition: none; }
}
</style>
