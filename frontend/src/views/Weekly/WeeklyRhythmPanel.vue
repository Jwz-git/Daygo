<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklyPresentation } from '@/stores/weeklyPresentation'
import { hourLabel } from '@/stores/weeklyPresentation'

const props = defineProps<{ presentation: WeeklyPresentation }>()
const { t } = useI18n()
const duration = useDurationFormat()

/** Hour bars between the first and last hour with any tracked minutes. */
const bars = computed(() => {
  const rhythm = props.presentation.rhythm
  let first = -1
  let last = -1
  rhythm.forEach((slot, hour) => {
    if (slot.focusMinutes + slot.idleMinutes > 0) {
      if (first < 0) first = hour
      last = hour
    }
  })
  if (first < 0) return [] as { hour: number; focus: number; idle: number; total: number }[]
  const max = Math.max(
    ...rhythm.map((slot) => slot.focusMinutes + slot.idleMinutes),
    1,
  )
  return rhythm
    .slice(first, last + 1)
    .map((slot) => ({
      hour: slot.hour,
      focus: slot.focusMinutes,
      idle: slot.idleMinutes,
      total: (slot.focusMinutes + slot.idleMinutes) / max,
    }))
})

const legend = computed(() => [
  { key: 'focus', label: t('weekly.rhythm.focus'), className: 'rhythm__bar--focus' },
  { key: 'idle', label: t('weekly.rhythm.idle'), className: 'rhythm__bar--idle' },
])

const dayWidthStyle = (bar: { total: number }) => ({
  '--rhythm-height': `${Math.max(4, bar.total * 100)}%`,
})
</script>

<template>
  <LiquidGlassSurface
    intensity="air"
    as="section"
    class="rhythm"
    :aria-label="t('weekly.rhythm.title')"
  >
    <header class="rhythm__header">
      <div>
        <p>{{ t('weekly.rhythm.eyebrow') }}</p>
        <h2>{{ t('weekly.rhythm.title') }}</h2>
      </div>
      <div class="rhythm__legend">
        <span v-for="item in legend" :key="item.key" class="rhythm__legend-item">
          <i :class="item.className" />
          {{ item.label }}
        </span>
      </div>
    </header>

    <div v-if="bars.length" class="rhythm__chart" role="img" :aria-label="t('weekly.rhythm.chartAria')">
      <div
        v-for="bar in bars"
        :key="bar.hour"
        class="rhythm__column"
        :class="{ 'rhythm__column--peak': bar.hour === presentation.insights.peakHour }"
        :style="dayWidthStyle(bar)"
        :title="`${hourLabel(bar.hour)} · ${t('weekly.rhythm.focus')} ${duration(bar.focus)} · ${t('weekly.rhythm.idle')} ${duration(bar.idle)}`"
      >
        <span class="rhythm__bar rhythm__bar--idle" />
        <span class="rhythm__bar rhythm__bar--focus" />
        <span class="rhythm__hour">{{ String(bar.hour).padStart(2, '0') }}</span>
      </div>
    </div>
    <p v-else class="rhythm__empty">{{ t('weekly.rhythm.empty') }}</p>
  </LiquidGlassSurface>
</template>

<style scoped>
.rhythm { padding: 24px 26px 18px; }

.rhythm__header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 20px;
}

.rhythm__header p {
  display: inline-block;
  color: var(--dg-weekly-tag-text);
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.rhythm__header h2 {
  margin-top: 7px;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-serif);
  font-size: 24px;
  font-weight: 400;
  letter-spacing: 0;
}

.rhythm__legend {
  display: flex;
  gap: 14px;
}

.rhythm__legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-text-tertiary);
  font-size: 11px;
}

.rhythm__legend-item i {
  width: 10px;
  height: 10px;
  border-radius: 3px;
}

.rhythm__chart {
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 148px;
  margin-top: 20px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--dg-card-border);
}

.rhythm__column {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  height: var(--rhythm-height);
  min-width: 0;
  flex: 1;
  gap: 1px;
}

.rhythm__column--peak .rhythm__hour { color: var(--dg-accent-text); font-weight: 650; }

.rhythm__bar { display: block; width: 100%; border-radius: 2px 2px 0 0; }

.rhythm__bar--focus { flex: 10000; background: var(--dg-weekly-focus); }
.rhythm__bar--idle { flex: 1 1 auto; min-height: 2px; background: var(--dg-weekly-series-6); opacity: 0.55; }

.rhythm__hour {
  position: absolute;
  top: calc(100% + 6px);
  left: 50%;
  transform: translateX(-50%);
  color: var(--dg-text-muted);
  font-size: 9px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.rhythm__empty {
  padding: 30px 0 8px;
  color: var(--dg-text-tertiary);
  font-size: 12px;
  text-align: center;
}

@media (prefers-reduced-motion: no-preference) {
  .rhythm__column {
    transform-origin: 50% 100%;
    animation: rhythm-grow 520ms var(--dg-ease-glide) both;
  }
}

@keyframes rhythm-grow {
  from { transform: scaleY(0); }
}

@media (max-width: 640px) {
  .rhythm__legend { display: none; }
  .rhythm__hour { font-size: 8px; }
}
</style>
