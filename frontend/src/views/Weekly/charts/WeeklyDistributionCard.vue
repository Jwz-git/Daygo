<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { WeeklyPresentation } from '@/stores/weeklyPresentation'

import WeeklyChartCard from './WeeklyChartCard.vue'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'

/*
 * "Weekly distribution" (Dayflow WeeklyDonutSection): a ring of category
 * sectors with small angular gaps and rounded ends over a solid disc, the
 * week total at the centre, and a legend with each category's share.
 */
const props = defineProps<{ presentation: WeeklyPresentation; days: string[] }>()

const { t } = useI18n()
const labels = useWeeklyChartLabels(() => props.days)

const SIZE = 200
const CENTER = SIZE / 2
const OUTER = SIZE / 2 - 4
const INNER = OUTER * 0.75
const GAP_DEGREES = 1.5

const items = computed(() => props.presentation.categories.filter((category) => category.minutes > 0))
const total = computed(() => items.value.reduce((sum, category) => sum + category.minutes, 0))

function point(radius: number, degrees: number): string {
  const radians = ((degrees - 90) * Math.PI) / 180
  return `${(CENTER + radius * Math.cos(radians)).toFixed(3)} ${(CENTER + radius * Math.sin(radians)).toFixed(3)}`
}

const sectors = computed(() => {
  if (total.value <= 0) return []
  let cursor = 0
  return items.value.map((category) => {
    const sweep = (category.minutes / total.value) * 360
    const gap = items.value.length > 1 ? Math.min(GAP_DEGREES, sweep / 2) : 0
    const start = cursor + gap / 2
    const end = cursor + sweep - gap / 2
    cursor += sweep
    const large = end - start > 180 ? 1 : 0
    // A single category draws a full ring: two half arcs avoid a degenerate path.
    const path = items.value.length === 1
      ? `M${point(OUTER, 0)} A${OUTER} ${OUTER} 0 1 1 ${point(OUTER, 180)} A${OUTER} ${OUTER} 0 1 1 ${point(OUTER, 360)} `
        + `M${point(INNER, 0)} A${INNER} ${INNER} 0 1 0 ${point(INNER, 180)} A${INNER} ${INNER} 0 1 0 ${point(INNER, 360)} Z`
      : `M${point(OUTER, start)} A${OUTER} ${OUTER} 0 ${large} 1 ${point(OUTER, end)} `
        + `L${point(INNER, end)} A${INNER} ${INNER} 0 ${large} 0 ${point(INNER, start)} Z`
    return { name: category.name, colorHex: category.colorHex, path }
  })
})

const hours = computed(() => Math.floor(Math.round(total.value) / 60))
const minutes = computed(() => Math.round(total.value) % 60)

function share(value: number): string {
  return total.value > 0 ? `${Math.round((value / total.value) * 100)}%` : '0%'
}
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.distribution.title')">
    <div class="dist">
      <div class="dist__chart" role="img" :aria-label="t('weekly.charts.distribution.aria')">
        <svg :viewBox="`0 0 ${SIZE} ${SIZE}`" aria-hidden="true">
          <circle class="dist__disc" :cx="CENTER" :cy="CENTER" :r="OUTER + 2" />
          <path
            v-for="sector in sectors"
            :key="sector.name"
            :d="sector.path"
            :fill="sector.colorHex"
            :stroke="sector.colorHex"
            stroke-width="3"
            stroke-linejoin="round"
            fill-rule="evenodd"
            opacity="0.85"
          />
        </svg>
        <div class="dist__center">
          <span class="dist__total-label">{{ t('weekly.charts.distribution.total') }}</span>
          <strong>{{ t('common.duration.hours', { count: hours }) }}</strong>
          <strong>{{ t('common.duration.minutes', { count: minutes }) }}</strong>
        </div>
      </div>
      <ul class="dist__legend">
        <li v-for="category in items" :key="category.name">
          <i :style="{ background: category.colorHex }" aria-hidden="true"></i>
          <span class="dist__name">{{ labels.category(category.name) }}</span>
          <span class="dist__share">{{ share(category.minutes) }}</span>
        </li>
      </ul>
    </div>
  </WeeklyChartCard>
</template>

<style scoped>
.dist {
  display: flex;
  align-items: center;
  gap: 22px;
  min-height: 230px;
}

.dist__chart {
  position: relative;
  flex: none;
  width: 200px;
  height: 200px;
}

.dist__chart svg {
  display: block;
  width: 100%;
  height: 100%;
  overflow: visible;
}

.dist__disc {
  fill: var(--dg-wk-solid);
  filter: drop-shadow(0 0 5px var(--dg-wk-donut-shadow));
}

.dist__center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  pointer-events: none;
}

.dist__total-label {
  margin-bottom: 2px;
  color: var(--dg-wk-text-muted);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.dist__center strong {
  color: var(--dg-wk-text);
  font-family: var(--dg-font-serif);
  font-size: 16px;
  font-weight: 400;
  line-height: 1.15;
}

.dist__legend {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  margin: 0;
  padding: 0;
  list-style: none;
}

.dist__legend li {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 12px;
}

.dist__legend i {
  flex: none;
  width: 10px;
  height: 10px;
  border-radius: 1.5px;
}

.dist__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--dg-wk-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dist__share {
  flex: none;
  color: var(--dg-wk-text-secondary);
  font-variant-numeric: tabular-nums;
}
</style>
