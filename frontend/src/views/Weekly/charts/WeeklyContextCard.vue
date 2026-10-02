<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { WeeklyContextSnapshot } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
import WeeklyChartTooltip from './WeeklyChartTooltip.vue'
import { useChartPointer } from './useChartPointer'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'

/*
 * "Context shift and distractions comparison" (Dayflow
 * WeeklyContextChartsSection): when switches and distractions happened
 * between 10:00 and 18:00, how many of each per day, and the busiest day.
 * Hovering a day in either chart highlights that day in both and counts it.
 */
const props = defineProps<{ snapshot: WeeklyContextSnapshot; days: string[] }>()

const { t } = useI18n()
const labels = useWeeklyChartLabels(() => props.days)

const CONTEXT_COLOR = '#B097FF'
const DISTRACTION_COLOR = '#FF7C5A'

// Scatter geometry (viewBox units).
const SCATTER = { width: 300, height: 170, left: 34, right: 14, top: 6, bottom: 22 }
const rowHeight = (SCATTER.height - SCATTER.top - SCATTER.bottom) / 7
function scatterX(minute: number): number {
  const span = props.snapshot.end - props.snapshot.start
  return SCATTER.left + ((minute - props.snapshot.start) / span) * (SCATTER.width - SCATTER.left - SCATTER.right)
}
function scatterY(dayIndex: number): number {
  return SCATTER.top + rowHeight * dayIndex + rowHeight / 2
}
const scatterTicks = computed(() => {
  const ticks: number[] = []
  for (let minute = props.snapshot.start; minute <= props.snapshot.end; minute += 120) ticks.push(minute)
  return ticks
})

// Comparison geometry.
const BARS = { width: 300, height: 170, top: 16, bottom: 22, left: 6, right: 6 }
const maxCount = computed(() => Math.max(1, ...props.snapshot.days.flatMap((day) => [day.shifts, day.distracted])))
const groupWidth = (BARS.width - BARS.left - BARS.right) / 7
const barWidth = Math.min(12, groupWidth / 3)
function barHeight(count: number): number {
  return (count / maxCount.value) * (BARS.height - BARS.top - BARS.bottom)
}
function groupX(dayIndex: number): number {
  return BARS.left + groupWidth * dayIndex + groupWidth / 2
}

const insight = computed(() => {
  const busiest = props.snapshot.busiest
  if (busiest === null) return t('weekly.charts.context.insightNone')
  return t('weekly.charts.context.insight', {
    day: labels.weekday(busiest.dayIndex, 'long'),
    shifts: busiest.shifts,
    distracted: busiest.distracted,
  })
})

type ContextDay = WeeklyContextSnapshot['days'][number]

const pointer = useChartPointer<ContextDay>()
const activeDay = computed(() => pointer.hovered.value?.dayIndex ?? null)

function dayAt(dayIndex: number): ContextDay {
  return props.snapshot.days.find((day) => day.dayIndex === dayIndex) ?? { dayIndex, shifts: 0, distracted: 0 }
}

function dayState(dayIndex: number): Record<string, boolean> {
  return { 'is-active': activeDay.value === dayIndex, 'is-dim': activeDay.value !== null && activeDay.value !== dayIndex }
}
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.context.title')">
    <div class="ctx__legend">
      <span><i :style="{ background: CONTEXT_COLOR }"></i>{{ t('weekly.charts.context.shifts') }}</span>
      <span><i :style="{ background: DISTRACTION_COLOR }"></i>{{ t('weekly.charts.context.distractions') }}</span>
    </div>
    <div
      :ref="(el) => { pointer.container.value = el as HTMLElement | null }"
      class="ctx"
      role="img"
      :aria-label="t('weekly.charts.context.aria')"
    >
      <figure class="ctx__figure">
        <figcaption>{{ t('weekly.charts.context.distribution') }}</figcaption>
        <svg :viewBox="`0 0 ${SCATTER.width} ${SCATTER.height}`" aria-hidden="true">
          <g v-for="dayIndex in 7" :key="`row-${dayIndex}`" class="ctx__day" :class="dayState(dayIndex - 1)">
            <rect
              class="ctx__hit"
              :x="0"
              :y="scatterY(dayIndex - 1) - rowHeight / 2"
              :width="SCATTER.width"
              :height="rowHeight"
              @pointermove="pointer.move($event, dayAt(dayIndex - 1))"
              @pointerleave="pointer.leave"
            />
            <line
              class="ctx__grid"
              :x1="SCATTER.left"
              :x2="SCATTER.width - SCATTER.right"
              :y1="scatterY(dayIndex - 1)"
              :y2="scatterY(dayIndex - 1)"
            />
            <text class="ctx__label" :x="SCATTER.left - 6" :y="scatterY(dayIndex - 1) + 3" text-anchor="end">
              {{ labels.weekday(dayIndex - 1) }}
            </text>
          </g>
          <text
            v-for="tick in scatterTicks"
            :key="`tick-${tick}`"
            class="ctx__label"
            :x="scatterX(tick)"
            :y="SCATTER.height - 6"
            text-anchor="middle"
          >{{ labels.hour(tick) }}</text>
          <circle
            v-for="(event, index) in snapshot.events"
            :key="`event-${index}`"
            class="ctx__dot"
            :class="dayState(event.dayIndex)"
            :cx="scatterX(event.minute)"
            :cy="scatterY(event.dayIndex) + (event.kind === 'context' ? -2.5 : 2.5)"
            r="3.5"
            :fill="event.kind === 'context' ? CONTEXT_COLOR : DISTRACTION_COLOR"
          />
        </svg>
      </figure>
      <figure class="ctx__figure">
        <figcaption>{{ t('weekly.charts.context.comparison') }}</figcaption>
        <svg :viewBox="`0 0 ${BARS.width} ${BARS.height}`" aria-hidden="true">
          <line
            class="ctx__axis"
            :x1="BARS.left"
            :x2="BARS.width - BARS.right"
            :y1="BARS.height - BARS.bottom"
            :y2="BARS.height - BARS.bottom"
          />
          <g
            v-for="day in snapshot.days"
            :key="`group-${day.dayIndex}`"
            class="ctx__day"
            :class="dayState(day.dayIndex)"
            @pointermove="pointer.move($event, day)"
            @pointerleave="pointer.leave"
          >
            <rect
              class="ctx__hit"
              :x="groupX(day.dayIndex) - groupWidth / 2"
              :y="0"
              :width="groupWidth"
              :height="BARS.height"
            />
            <rect
              class="ctx__bar"
              :x="groupX(day.dayIndex) - barWidth - 1"
              :y="BARS.height - BARS.bottom - barHeight(day.shifts)"
              :width="barWidth"
              :height="barHeight(day.shifts)"
              rx="2"
              :fill="CONTEXT_COLOR"
            />
            <rect
              class="ctx__bar"
              :x="groupX(day.dayIndex) + 1"
              :y="BARS.height - BARS.bottom - barHeight(day.distracted)"
              :width="barWidth"
              :height="barHeight(day.distracted)"
              rx="2"
              :fill="DISTRACTION_COLOR"
            />
            <text
              v-if="day.shifts + day.distracted > 0"
              class="ctx__value"
              :x="groupX(day.dayIndex)"
              :y="BARS.height - BARS.bottom - Math.max(barHeight(day.shifts), barHeight(day.distracted)) - 4"
              text-anchor="middle"
            >{{ day.shifts }}·{{ day.distracted }}</text>
            <text class="ctx__label" :x="groupX(day.dayIndex)" :y="BARS.height - 6" text-anchor="middle">
              {{ labels.weekday(day.dayIndex) }}
            </text>
          </g>
        </svg>
      </figure>
      <WeeklyChartTooltip :visible="pointer.hovered.value !== null" :x="pointer.x.value" :y="pointer.y.value">
        <template v-if="pointer.hovered.value">
          <b>{{ labels.weekday(pointer.hovered.value.dayIndex, 'long') }}</b>
          <span><i class="ctx__swatch" :style="{ background: CONTEXT_COLOR }"></i>{{ t('weekly.charts.tooltip.shifts', { count: pointer.hovered.value.shifts }) }}</span>
          <span><i class="ctx__swatch" :style="{ background: DISTRACTION_COLOR }"></i>{{ t('weekly.charts.tooltip.distractions', { count: pointer.hovered.value.distracted }) }}</span>
        </template>
      </WeeklyChartTooltip>
    </div>
    <template #footer>{{ insight }}</template>
  </WeeklyChartCard>
</template>

<style scoped>
.ctx__legend {
  display: flex;
  gap: 18px;
  margin-bottom: 8px;
  color: var(--dg-wk-text-secondary);
  font-size: 12.5px;
  font-weight: 500;
}

.ctx__legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.ctx__legend i {
  width: 9px;
  height: 9px;
  border-radius: 50%;
}

.ctx {
  position: relative;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.ctx__figure {
  min-width: 0;
  margin: 0;
}

.ctx__figure figcaption {
  margin-bottom: 4px;
  color: var(--dg-wk-text-muted);
  font-size: 12px;
  font-weight: 600;
}

.ctx__figure svg {
  display: block;
  width: 100%;
  height: auto;
}

.ctx__grid {
  stroke: var(--dg-wk-divider);
  stroke-width: 0.5;
  stroke-dasharray: 2 3;
}

.ctx__axis {
  stroke: var(--dg-wk-axis-soft);
  stroke-width: 1;
}

.ctx__label {
  fill: var(--dg-wk-text-secondary);
  font-size: 10.5px;
  font-weight: 500;
  transition: fill 200ms ease-out;
}

.ctx__value {
  fill: var(--dg-wk-text);
  font-size: 10px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
}

.ctx__hit {
  fill: transparent;
  cursor: default;
}

.ctx__day,
.ctx__dot {
  transition: opacity 220ms cubic-bezier(0.22, 1, 0.36, 1);
}

.ctx__day.is-dim,
.ctx__dot.is-dim {
  opacity: 0.3;
}

.ctx__day.is-active .ctx__label {
  fill: var(--dg-wk-text);
  font-weight: 650;
}

.ctx__day.is-active .ctx__hit {
  fill: var(--dg-wk-row-fill);
  fill-opacity: 0.7;
}

.ctx__dot {
  fill-opacity: 0.85;
  pointer-events: none;
  transform-box: fill-box;
  transform-origin: center;
  transition:
    opacity 220ms cubic-bezier(0.22, 1, 0.36, 1),
    transform 240ms cubic-bezier(0.22, 1, 0.36, 1);
}

.ctx__dot.is-active {
  fill-opacity: 1;
  transform: scale(1.35);
}

.ctx__bar {
  pointer-events: none;
}

.ctx__swatch {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-right: 6px;
  border-radius: 50%;
}

@media (prefers-reduced-motion: reduce) {
  .ctx__day,
  .ctx__dot { transition: none; }
  .ctx__dot.is-active { transform: none; }
}

@media (max-width: 760px) {
  .ctx { grid-template-columns: minmax(0, 1fr); }
}
</style>
