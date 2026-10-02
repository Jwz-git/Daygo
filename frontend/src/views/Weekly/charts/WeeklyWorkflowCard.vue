<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useDurationFormat } from '@/lib/duration'
import { hourTicks, type WeeklyWorkflowSnapshot } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'

/*
 * "Your workflow this week" (Dayflow WeeklyWorkflowSection): one row per day,
 * one rounded cell per 15 minutes in the dominant category's colour, the
 * week's top category totals in the footer.
 */
const props = defineProps<{ snapshot: WeeklyWorkflowSnapshot; days: string[] }>()

const { t } = useI18n()
const formatDuration = useDurationFormat()
const labels = useWeeklyChartLabels(() => props.days)

const CELL = 13
const GAP = 2
const LABEL_WIDTH = 36
const AXIS_HEIGHT = 20

const columns = computed(() => props.snapshot.rows[0]?.length ?? 0)
const width = computed(() => LABEL_WIDTH + columns.value * (CELL + GAP))
const height = 7 * (CELL + GAP) + AXIS_HEIGHT

function cellX(slot: number): number {
  return LABEL_WIDTH + slot * (CELL + GAP)
}

const ticks = computed(() =>
  hourTicks(props.snapshot.start, props.snapshot.end).map((minute) => ({
    minute,
    x: cellX((minute - props.snapshot.start) / props.snapshot.slotMinutes),
  })),
)
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.workflow.title')">
    <div class="wf" role="img" :aria-label="t('weekly.charts.workflow.aria')">
      <svg :viewBox="`0 0 ${width} ${height}`" aria-hidden="true" preserveAspectRatio="xMinYMin meet">
        <g v-for="(row, dayIndex) in snapshot.rows" :key="`row-${dayIndex}`">
          <text class="wf__label" :x="LABEL_WIDTH - 8" :y="dayIndex * (CELL + GAP) + CELL - 3" text-anchor="end">
            {{ labels.weekday(dayIndex) }}
          </text>
          <rect
            v-for="(cell, slot) in row"
            :key="slot"
            :x="cellX(slot)"
            :y="dayIndex * (CELL + GAP)"
            :width="CELL"
            :height="CELL"
            rx="2.5"
            :class="{ 'wf__cell--empty': cell.colorHex === null }"
            :fill="cell.colorHex ?? undefined"
            :fill-opacity="cell.colorHex === null ? undefined : 0.35 + 0.65 * cell.occupancy"
          />
        </g>
        <text
          v-for="tick in ticks"
          :key="`tick-${tick.minute}`"
          class="wf__label"
          :x="tick.x"
          :y="7 * (CELL + GAP) + 13"
          text-anchor="middle"
        >{{ labels.hour(tick.minute) }}</text>
      </svg>
    </div>
    <template #footer>
      <div class="wf__totals">
        <span class="wf__totals-title">{{ t('weekly.charts.workflow.total') }}</span>
        <span v-for="total in snapshot.totals" :key="total.name" class="wf__total">
          <i :style="{ background: total.colorHex }" aria-hidden="true"></i>
          {{ labels.category(total.name) }}
          <b>{{ formatDuration(total.minutes) }}</b>
        </span>
      </div>
    </template>
  </WeeklyChartCard>
</template>

<style scoped>
.wf {
  overflow-x: auto;
}

.wf svg {
  display: block;
  width: 100%;
  min-width: 560px;
  height: auto;
}

.wf__cell--empty {
  fill: var(--dg-wk-empty-cell);
}

.wf__label {
  fill: var(--dg-wk-text-secondary);
  font-size: 9px;
}

.wf__totals {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 16px;
}

.wf__totals-title {
  color: var(--dg-wk-text-secondary);
}

.wf__total {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-wk-text);
}

.wf__total i {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}

.wf__total b {
  color: var(--dg-wk-text-secondary);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
</style>
