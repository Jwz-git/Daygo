<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { useDurationFormat } from '@/lib/duration'
import type { WeeklyWorkflowCell, WeeklyWorkflowSnapshot } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
import WeeklyChartTooltip from './WeeklyChartTooltip.vue'
import { svgPoint, useChartPointer } from './useChartPointer'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'
import { useWeeklyGrid } from './useWeeklyGrid'

/*
 * "Your workflow this week" (Dayflow WeeklyWorkflowSection): one row per day,
 * one rounded cell per 15 minutes in the dominant category's colour, the
 * week's top category totals in the footer. Hovering a cell outlines it,
 * recedes the other days and names the slot; hovering a total highlights
 * that category's cells.
 */
const props = defineProps<{ snapshot: WeeklyWorkflowSnapshot; days: string[] }>()

const { t } = useI18n()
const formatDuration = useDurationFormat()
const labels = useWeeklyChartLabels(() => props.days)

const CELL = 13
const GAP = 2
const LABEL_WIDTH = 40
const AXIS_HEIGHT = 22

const columns = computed(() => props.snapshot.rows[0]?.length ?? 0)
const { container: gridContainer, width, step, ticks } = useWeeklyGrid(
  () => ({ start: props.snapshot.start, end: props.snapshot.end, columns: columns.value }),
  LABEL_WIDTH,
)
const height = 7 * (CELL + GAP) + AXIS_HEIGHT
const cellWidth = computed(() => step.value - GAP)

function cellX(slot: number): number {
  return LABEL_WIDTH + slot * step.value
}

interface HoveredCell {
  day: number
  slot: number
  cell: WeeklyWorkflowCell
}

const pointer = useChartPointer<HoveredCell>()

// One listener on the svg, mapped to the grid, so gaps between cells never flicker.
function onPointerMove(event: PointerEvent): void {
  const point = svgPoint(event)
  const day = point === null ? -1 : Math.floor(point.y / (CELL + GAP))
  const slot = point === null ? -1 : Math.floor((point.x - LABEL_WIDTH) / step.value)
  const cell = props.snapshot.rows[day]?.[slot]
  if (cell !== undefined) pointer.move(event, { day, slot, cell })
  else pointer.leave()
}

function slotRange(slot: number): string {
  const start = props.snapshot.start + slot * props.snapshot.slotMinutes
  return `${labels.clock(start)} – ${labels.clock(start + props.snapshot.slotMinutes)}`
}

// Footer totals highlight their category's cells across the week.
const legendCategory = ref<string | null>(null)
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.workflow.title')">
    <div
      :ref="(el) => { pointer.container.value = el as HTMLElement | null }"
      class="wf"
      :class="{ 'has-cell': pointer.hovered.value !== null, 'has-category': legendCategory !== null }"
      role="img"
      :aria-label="t('weekly.charts.workflow.aria')"
    >
      <div ref="gridContainer" class="wf__scroll">
        <svg
          :width="width"
          :height="height"
          :viewBox="`0 0 ${width} ${height}`"
          aria-hidden="true"
          preserveAspectRatio="xMinYMin meet"
          @pointermove="onPointerMove"
          @pointerleave="pointer.leave"
        >
          <g
            v-for="(row, dayIndex) in snapshot.rows"
            :key="`row-${dayIndex}`"
            class="wf__row"
            :class="{ 'is-current': pointer.hovered.value?.day === dayIndex }"
          >
            <text class="wf__label" :x="LABEL_WIDTH - 8" :y="dayIndex * (CELL + GAP) + CELL - 2.5" text-anchor="end">
              {{ labels.weekday(dayIndex) }}
            </text>
            <rect
              v-for="(cell, slot) in row"
              :key="slot"
              class="wf__cell"
              :class="{
                'wf__cell--empty': cell.colorHex === null,
                'is-match': cell.category !== null && cell.category === legendCategory,
              }"
              :x="cellX(slot)"
              :y="dayIndex * (CELL + GAP)"
              :width="cellWidth"
              :height="CELL"
              rx="2.5"
              :fill="cell.colorHex ?? undefined"
              :fill-opacity="cell.colorHex === null ? undefined : 0.35 + 0.65 * cell.occupancy"
            />
          </g>
          <rect
            v-if="pointer.hovered.value"
            class="wf__cursor"
            :style="{
              transform: `translate(${cellX(pointer.hovered.value.slot) - 1}px, ${pointer.hovered.value.day * (CELL + GAP) - 1}px)`,
            }"
            :width="cellWidth + 2"
            :height="CELL + 2"
            rx="3.5"
          />
          <text
            v-for="tick in ticks"
            :key="`tick-${tick.minute}`"
            class="wf__label"
            :x="tick.x"
            :y="7 * (CELL + GAP) + 15"
            text-anchor="middle"
          >{{ labels.hour(tick.minute) }}</text>
        </svg>
      </div>
      <WeeklyChartTooltip :visible="pointer.hovered.value !== null" :x="pointer.x.value" :y="pointer.y.value">
        <template v-if="pointer.hovered.value">
          <b>{{ labels.weekday(pointer.hovered.value.day, 'long') }} · {{ slotRange(pointer.hovered.value.slot) }}</b>
          <span v-if="pointer.hovered.value.cell.category">
            {{ labels.category(pointer.hovered.value.cell.category) }} · {{ formatDuration(pointer.hovered.value.cell.minutes) }}
          </span>
          <span v-else>{{ t('weekly.charts.tooltip.noRecord') }}</span>
        </template>
      </WeeklyChartTooltip>
    </div>
    <template #footer>
      <div class="wf__totals">
        <span class="wf__totals-title">{{ t('weekly.charts.workflow.total') }}</span>
        <span
          v-for="total in snapshot.totals"
          :key="total.name"
          class="wf__total"
          :class="{ 'is-dim': legendCategory !== null && legendCategory !== total.name }"
          @pointerenter="legendCategory = total.name"
          @pointerleave="legendCategory = null"
        >
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
  position: relative;
}

.wf__scroll {
  overflow-x: auto;
}

.wf svg {
  display: block;
}

.wf__row {
  transition: opacity 220ms cubic-bezier(0.22, 1, 0.36, 1);
}

.wf__cell {
  transition: opacity 220ms cubic-bezier(0.22, 1, 0.36, 1);
}

.wf__cell--empty {
  fill: var(--dg-wk-empty-cell);
}

/* Hovering a cell recedes the other days; hovering a total recedes the other categories. */
.wf.has-cell .wf__row:not(.is-current) {
  opacity: 0.5;
}

.wf.has-category .wf__cell:not(.is-match):not(.wf__cell--empty) {
  opacity: 0.18;
}

.wf__cursor {
  fill: none;
  stroke: var(--dg-wk-text);
  stroke-width: 1.5;
  pointer-events: none;
  transition: transform 110ms cubic-bezier(0.22, 1, 0.36, 1);
}

.wf__label {
  fill: var(--dg-wk-text-secondary);
  font-size: 11px;
  font-weight: 500;
  transition: fill 200ms ease-out;
}

.wf__row.is-current .wf__label {
  fill: var(--dg-wk-text);
  font-weight: 650;
}

.wf__totals {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 16px;
}

.wf__totals-title {
  color: var(--dg-wk-text-secondary);
  font-weight: 500;
}

.wf__total {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-wk-text);
  font-weight: 500;
  cursor: default;
  transition: opacity 220ms ease-out;
}

.wf__total.is-dim {
  opacity: 0.4;
}

.wf__total i {
  width: 9px;
  height: 9px;
  border-radius: 2px;
}

.wf__total b {
  color: var(--dg-wk-text-secondary);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

@media (prefers-reduced-motion: reduce) {
  .wf__row,
  .wf__cell,
  .wf__cursor,
  .wf__total { transition: none; }
}
</style>
