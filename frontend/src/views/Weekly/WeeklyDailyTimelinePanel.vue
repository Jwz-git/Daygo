<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklyPresentation } from '@/stores/weeklyPresentation'
import { hourLabel, localMinuteOfDay } from '@/stores/weeklyPresentation'

const props = defineProps<{ presentation: WeeklyPresentation }>()
const { t, locale } = useI18n()

const duration = useDurationFormat()

const windowStart = computed(() => props.presentation.windowStartMinute)
const windowSpan = computed(() =>
  Math.max(1, props.presentation.windowEndMinute - props.presentation.windowStartMinute))

const dayNames = computed(() => {
  const format = new Intl.DateTimeFormat(locale.value, { weekday: 'short', timeZone: 'UTC' })
  return (day: string) => (day ? format.format(new Date(`${day}T12:00:00Z`)) : '—')
})

const axisTicks = computed(() => {
  const ticks: { minute: number; label: string }[] = []
  const step = windowSpan.value > 12 * 60 ? 180 : 120
  const start = Math.ceil(windowStart.value / step) * step
  for (let minute = start; minute <= props.presentation.windowEndMinute; minute += step) {
    ticks.push({ minute, label: hourLabel(minute / 60) })
  }
  return ticks
})

function segmentStyle(segment: { startMinute: number; endMinute: number; colorHex: string }) {
  const left = ((segment.startMinute - windowStart.value) / windowSpan.value) * 100
  const width = Math.max(0.4, ((segment.endMinute - segment.startMinute) / windowSpan.value) * 100)
  return {
    left: `${left}%`,
    width: `${width}%`,
    background: segment.colorHex,
  }
}

function segmentTitle(day: string, segment: { category: string; minutes: number; startMinute: number; endMinute: number; isIdle: boolean }) {
  const clock = (minute: number) => hourLabel(Math.floor(minute / 60))
  return [
    dayNames.value(day),
    `${clock(segment.startMinute)}–${clock(segment.endMinute)}`,
    `${categoryLabel(segment.category, t)} · ${duration(segment.minutes)}`,
    segment.isIdle ? t('weekly.daily.idleTag') : '',
  ].filter(Boolean).join(' · ')
}
</script>

<template>
  <LiquidGlassSurface
    intensity="air"
    as="section"
    class="daily"
    :aria-label="t('weekly.daily.title')"
  >
    <header class="daily__header">
      <div>
        <p>{{ t('weekly.daily.eyebrow') }}</p>
        <h2>{{ t('weekly.daily.title') }}</h2>
      </div>
      <span>{{ t('weekly.daily.hint') }}</span>
    </header>

    <div class="daily__chart">
      <div class="daily__axis" aria-hidden="true">
        <span
          v-for="tick in axisTicks"
          :key="tick.minute"
          class="daily__tick"
          :style="{ left: `${((tick.minute - windowStart) / windowSpan) * 100}%` }"
        >{{ tick.label }}</span>
      </div>

      <div class="daily__rows">
        <div class="daily__guides" aria-hidden="true">
          <span
            v-for="tick in axisTicks"
            :key="`guide-${tick.minute}`"
            :style="{ left: `${((tick.minute - windowStart) / windowSpan) * 100}%` }"
          />
        </div>

        <div v-for="day in presentation.days" :key="day.weekday" class="daily__row">
          <span class="daily__day">{{ dayNames(day.day) }}</span>
          <div
            class="daily__track"
            :title="day.segments.length ? undefined : t('weekly.daily.noActivity')"
          >
            <span
              v-for="(segment, index) in day.segments"
              :key="index"
              class="daily__segment"
              :class="{ 'daily__segment--idle': segment.isIdle }"
              :style="segmentStyle(segment)"
              :title="segmentTitle(day.day, segment)"
            />
          </div>
          <span class="daily__total">{{ day.trackedMinutes > 0 ? duration(day.trackedMinutes) : '' }}</span>
        </div>
      </div>
    </div>

    <div class="daily__legend">
      <span v-for="category in presentation.categories" :key="category.name">
        <i :style="{ background: category.colorHex }" />
        {{ categoryLabel(category.name, t) }}
      </span>
    </div>
  </LiquidGlassSurface>
</template>

<style scoped>
.daily {
  padding: 24px 26px 18px;
}

.daily__header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 22px;
}

.daily__header p {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--dg-weekly-tag-fill);
  color: var(--dg-weekly-tag-text);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.09em;
  text-transform: uppercase;
}

.daily__header h2 {
  margin-top: 7px;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-serif);
  font-size: 24px;
  font-weight: 400;
  letter-spacing: 0;
}

.daily__header > span { color: var(--dg-text-muted); font-size: 10px; }

.daily__chart {
  --label-w: 40px;
  --total-w: 48px;
  --col-gap: 12px;
}
.daily__axis {
  position: relative;
  height: 15px;
  margin: 0 calc(var(--total-w) + var(--col-gap)) 6px calc(var(--label-w) + var(--col-gap));
}

.daily__tick {
  position: absolute;
  bottom: 0;
  transform: translateX(-50%);
  color: var(--dg-text-muted);
  font-size: 9px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.daily__tick:first-child { transform: none; }
.daily__tick:last-child { transform: translateX(-100%); }

.daily__rows {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.daily__guides {
  position: absolute;
  z-index: 0;
  top: 0;
  bottom: 0;
  left: calc(var(--label-w) + var(--col-gap));
  right: calc(var(--total-w) + var(--col-gap));
  pointer-events: none;
}

.daily__guides span {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  transform: translateX(-0.5px);
  background: var(--dg-daily-grid-line);
}
.daily__row {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: var(--label-w) minmax(0, 1fr) var(--total-w);
  align-items: center;
  gap: 0 var(--col-gap);
}

.daily__day {
  color: var(--dg-text-secondary);
  font-size: 11px;
  white-space: nowrap;
}

.daily__track {
  position: relative;
  height: 24px;
  overflow: hidden;
  border-radius: 6px;
  background: var(--dg-weekly-bar-track);
}

.daily__segment {
  position: absolute;
  top: 0;
  bottom: 0;
  border-radius: 5px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16);
}

.daily__segment--idle {
  opacity: 0.42;
  box-shadow: none;
}

.daily__total {
  color: var(--dg-text-muted);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
  text-align: right;
  white-space: nowrap;
}
.daily__legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  margin-top: 18px;
  padding-top: 12px;
  border-top: 1px solid var(--dg-card-border);
}

.daily__legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-text-secondary);
  font-size: 10px;
}

.daily__legend i {
  width: 10px;
  height: 10px;
  border-radius: 3px;
}

@media (prefers-reduced-motion: no-preference) {
  .daily__segment {
    transform-origin: 0 50%;
    animation: daily-grow 560ms var(--dg-ease-glide) both;
  }
}

@keyframes daily-grow {
  from { transform: scaleX(0); }
}

@media (max-width: 640px) {
  .daily__chart { --label-w: 34px; --total-w: 0px; --col-gap: 10px; }
  .daily__total { display: none; }
  .daily__legend { gap: 4px 12px; }
}
</style>
