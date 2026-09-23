<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklyDaySegment, WeeklyPresentation } from '@/stores/weeklyPresentation'
import { hourLabel } from '@/stores/weeklyPresentation'

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

interface DailyRun {
  category: string
  colorHex: string
  isIdle: boolean
  startMinute: number
  endMinute: number
  minutes: number
  joinLeft: boolean
}

// Bridge sub-gaps this small (minutes) between same-category segments so a
// continuous stretch of one category reads as a single run, not confetti.
const MERGE_GAP = 2

// Segments arrive sorted by startMinute. Fold consecutive same-category (same
// idle-ness) touching/overlapping segments into one run, then flag work runs
// that directly abut a *different* work run so we can hairline-divide them.
function buildRuns(segments: WeeklyDaySegment[]): DailyRun[] {
  const runs: DailyRun[] = []
  for (const seg of segments) {
    const last = runs[runs.length - 1]
    if (last && last.category === seg.category && last.isIdle === seg.isIdle
      && seg.startMinute - last.endMinute <= MERGE_GAP) {
      last.endMinute = Math.max(last.endMinute, seg.endMinute)
      last.minutes += seg.minutes
    } else {
      runs.push({
        category: seg.category,
        colorHex: seg.colorHex,
        isIdle: seg.isIdle,
        startMinute: seg.startMinute,
        endMinute: seg.endMinute,
        minutes: seg.minutes,
        joinLeft: false,
      })
    }
  }
  for (let i = 1; i < runs.length; i += 1) {
    // Hairline only where two work runs touch with no gap; idle sits beneath
    // work (z-index), and a real gap leaves the track showing instead.
    runs[i].joinLeft = !runs[i].isIdle && !runs[i - 1].isIdle
      && runs[i].startMinute - runs[i - 1].endMinute <= 0.5
  }
  return runs
}

const rows = computed(() => {
  const start = windowStart.value
  const span = windowSpan.value
  const name = dayNames.value
  return props.presentation.days.map((day) => {
    const runs = buildRuns(day.segments).map((run) => ({
      ...run,
      style: {
        left: `${((run.startMinute - start) / span) * 100}%`,
        width: `${Math.max(0.6, ((run.endMinute - run.startMinute) / span) * 100)}%`,
        '--run-color': run.colorHex,
      },
    }))
    return {
      key: day.weekday,
      name: name(day.day),
      total: day.trackedMinutes > 0 ? duration(day.trackedMinutes) : '',
      hasActivity: runs.length > 0,
      runs,
    }
  })
})

function runTitle(dayName: string, run: DailyRun): string {
  const clock = (minute: number) => hourLabel(Math.floor(minute / 60))
  return [
    dayName,
    `${clock(run.startMinute)}–${clock(run.endMinute)}`,
    `${categoryLabel(run.category, t)} · ${duration(run.minutes)}`,
    run.isIdle ? t('weekly.daily.idleTag') : '',
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

        <div
          v-for="(row, index) in rows"
          :key="row.key"
          class="daily__row"
          :style="{ '--row-i': index }"
        >
          <span class="daily__day">{{ row.name }}</span>
          <div
            class="daily__track"
            role="img"
            :aria-label="`${row.name} · ${row.hasActivity ? row.total : t('weekly.daily.noActivity')}`"
          >
            <span v-if="!row.hasActivity" class="daily__empty">{{ t('weekly.daily.noActivity') }}</span>
            <span
              v-for="(run, i) in row.runs"
              :key="i"
              class="daily__run"
              :class="{ 'daily__run--idle': run.isIdle, 'daily__run--join': run.joinLeft }"
              :style="run.style"
              :title="runTitle(row.name, run)"
            />
          </div>
          <span class="daily__total">{{ row.total }}</span>
        </div>
      </div>
    </div>

    <div class="daily__legend">
      <span
        v-for="category in presentation.categories"
        :key="category.name"
        :class="{ 'daily__legend-item--idle': category.name === 'Idle' }"
      >
        <i :style="{ '--legend-color': category.colorHex }" />
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
  --total-w: 52px;
  --col-gap: 14px;
}

.daily__axis {
  position: relative;
  height: 15px;
  margin: 0 calc(var(--total-w) + var(--col-gap)) 8px calc(var(--label-w) + var(--col-gap));
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
  gap: 8px;
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

/* The whole lane is one clipped bar: rounded ends read a full day as a single
   continuous strip; runs inside butt-join with square cuts, and a recessed
   groove overlay makes the colours sit inside the channel. */
.daily__track {
  position: relative;
  height: 22px;
  overflow: hidden;
  border-radius: 7px;
  background: var(--dg-weekly-bar-track);
}

.daily__track::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 3;
  border-radius: inherit;
  box-shadow: var(--dg-daily-track-groove);
  pointer-events: none;
}

.daily__run {
  position: absolute;
  top: 0;
  bottom: 0;
  z-index: 2;
  background: var(--run-color);
}

/* Hairline only where a work run abuts a different work run with no gap. */
.daily__run--join {
  box-shadow: inset 0.75px 0 0 var(--dg-daily-run-divider);
}

/* Idle: a quiet solid diagonal hatch beneath work, never a translucent wash. */
.daily__run--idle {
  z-index: 1;
  background:
    repeating-linear-gradient(-45deg, var(--dg-daily-idle-line) 0 1px, transparent 1px 5px),
    var(--dg-daily-idle-fill);
}

.daily__empty {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--dg-text-muted);
  font-size: 9px;
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
  background: var(--legend-color);
}

.daily__legend-item--idle i {
  background:
    repeating-linear-gradient(-45deg, var(--dg-daily-idle-line) 0 1px, transparent 1px 4px),
    var(--dg-daily-idle-fill);
}

@media (prefers-reduced-motion: no-preference) {
  .daily__run {
    transform-origin: 0 50%;
    animation: daily-grow 520ms var(--dg-ease-glide) both;
    animation-delay: calc(var(--row-i, 0) * 40ms);
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


