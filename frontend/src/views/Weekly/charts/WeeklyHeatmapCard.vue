<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { hourTicks, type WeeklyHeatmapSnapshot } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'

/*
 * "Focus and distraction heat map" (Dayflow WeeklyFocusHeatmapSection): one
 * row per day, one cell per 5 minutes, coloured from the bucket score:
 * negative runs soft lavender to blue (focused), positive soft peach to
 * orange (distracted), near zero stays the neutral empty-cell colour.
 */
const props = defineProps<{ snapshot: WeeklyHeatmapSnapshot; days: string[] }>()

const { t } = useI18n()
const labels = useWeeklyChartLabels(() => props.days)

const FOCUS_SOFT = [0xe3, 0xdb, 0xfd]
const FOCUS_DARK = [0x42, 0x76, 0xe9]
const DISTRACTION_SOFT = [0xf8, 0xd1, 0xca]
const DISTRACTION_DARK = [0xfc, 0x76, 0x45]
const NEUTRAL_THRESHOLD = 0.045

function mix(from: number[], to: number[], amount: number): string {
  const channel = (index: number) => Math.round(from[index] + (to[index] - from[index]) * amount)
  return `rgb(${channel(0)}, ${channel(1)}, ${channel(2)})`
}

function cellColor(score: number): string | null {
  if (Math.abs(score) < NEUTRAL_THRESHOLD) return null
  const amount = Math.min(1, Math.abs(score))
  return score < 0 ? mix(FOCUS_SOFT, FOCUS_DARK, amount) : mix(DISTRACTION_SOFT, DISTRACTION_DARK, amount)
}

const CELL_WIDTH = 6
const CELL_HEIGHT = 12
const ROW_GAP = 2
const LABEL_WIDTH = 30
const AXIS_HEIGHT = 18

const columns = computed(() => props.snapshot.rows[0]?.length ?? 0)
const width = computed(() => LABEL_WIDTH + columns.value * CELL_WIDTH)
const height = 7 * (CELL_HEIGHT + ROW_GAP) + AXIS_HEIGHT

const ticks = computed(() =>
  hourTicks(props.snapshot.start, props.snapshot.end).map((minute) => ({
    minute,
    x: LABEL_WIDTH + ((minute - props.snapshot.start) / props.snapshot.bucketMinutes) * CELL_WIDTH,
  })),
)
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.heatmap.title')">
    <div class="hm__legend" aria-hidden="true">
      <span>{{ t('weekly.charts.heatmap.focused') }}</span>
      <i class="hm__legend-bar"></i>
      <span>{{ t('weekly.charts.heatmap.distracted') }}</span>
    </div>
    <div class="hm" role="img" :aria-label="t('weekly.charts.heatmap.aria')">
      <svg :viewBox="`0 0 ${width} ${height}`" aria-hidden="true" preserveAspectRatio="xMinYMin meet">
        <g v-for="(row, dayIndex) in snapshot.rows" :key="`row-${dayIndex}`">
          <text
            class="hm__label"
            :x="LABEL_WIDTH - 6"
            :y="dayIndex * (CELL_HEIGHT + ROW_GAP) + CELL_HEIGHT - 3"
            text-anchor="end"
          >{{ labels.weekday(dayIndex) }}</text>
          <rect
            v-for="(score, bucket) in row"
            :key="bucket"
            :x="LABEL_WIDTH + bucket * CELL_WIDTH"
            :y="dayIndex * (CELL_HEIGHT + ROW_GAP)"
            :width="CELL_WIDTH - 0.5"
            :height="CELL_HEIGHT"
            :class="{ 'hm__cell--neutral': cellColor(score) === null }"
            :fill="cellColor(score) ?? undefined"
          />
        </g>
        <text
          v-for="tick in ticks"
          :key="`tick-${tick.minute}`"
          class="hm__label"
          :x="tick.x"
          :y="7 * (CELL_HEIGHT + ROW_GAP) + 12"
          text-anchor="middle"
        >{{ labels.hour(tick.minute) }}</text>
      </svg>
    </div>
  </WeeklyChartCard>
</template>

<style scoped>
.hm__legend {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin: -34px 0 14px;
  color: var(--dg-wk-text-secondary);
  font-size: 11px;
}

.hm__legend-bar {
  width: 180px;
  height: 8px;
  border-radius: 2px;
  background: linear-gradient(90deg, #4276e9, #e3dbfd 48%, #f8d1ca 52%, #fc7645);
}

.hm {
  overflow-x: auto;
}

.hm svg {
  display: block;
  width: 100%;
  min-width: 560px;
  height: auto;
}

.hm__cell--neutral {
  fill: var(--dg-wk-empty-cell);
}

.hm__label {
  fill: var(--dg-wk-text-secondary);
  font-size: 9px;
}

@media (max-width: 760px) {
  .hm__legend { margin-top: 0; justify-content: flex-start; }
}
</style>
