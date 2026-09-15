<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklyPresentation } from '@/stores/weeklyPresentation'

const props = defineProps<{ presentation: WeeklyPresentation }>()
const { t, locale } = useI18n()

const duration = useDurationFormat()

const dayLabel = computed(() => (day: string) => {
  if (!day) return t('weekly.insights.none')
  return new Intl.DateTimeFormat(locale.value, { weekday: 'long', timeZone: 'UTC' })
    .format(new Date(`${day}T12:00:00Z`))
})

const peakLabel = computed(() => {
  const hour = props.presentation.insights.peakHour
  if (hour < 0 || hour > 23) return t('weekly.insights.none')
  return `${String(hour).padStart(2, '0')}:00`
})

const cards = computed(() => [
  {
    key: 'active',
    label: t('weekly.insights.activeDays'),
    value: t('weekly.insights.daysCount', { count: props.presentation.insights.activeDays }),
    detail: t('weekly.insights.ofSeven'),
  },
  {
    key: 'longest',
    label: t('weekly.insights.longestFocus'),
    value: duration(props.presentation.insights.longestFocusMinutes),
    detail: props.presentation.insights.longestFocusDay
      ? dayLabel.value(props.presentation.insights.longestFocusDay)
      : t('weekly.insights.none'),
  },
  {
    key: 'peak',
    label: t('weekly.insights.peakHour'),
    value: peakLabel.value,
    detail: duration(props.presentation.insights.peakHourMinutes),
  },
  {
    key: 'avg',
    label: t('weekly.insights.avgFocus'),
    value: duration(props.presentation.insights.avgDailyFocusMinutes),
    detail: t('weekly.insights.perActiveDay'),
  },
  {
    key: 'busiest',
    label: t('weekly.insights.busiestDay'),
    value: props.presentation.insights.mostActiveDay
      ? dayLabel.value(props.presentation.insights.mostActiveDay)
      : t('weekly.insights.none'),
    detail: duration(props.presentation.insights.mostActiveDayMinutes),
  },
])
</script>

<template>
  <LiquidGlassSurface
    intensity="air"
    as="section"
    class="insights"
    :aria-label="t('weekly.insights.title')"
  >
    <div v-for="card in cards" :key="card.key" class="insights__card">
      <span class="insights__label">{{ card.label }}</span>
      <strong class="insights__value">{{ card.value }}</strong>
      <span class="insights__detail">{{ card.detail }}</span>
    </div>
  </LiquidGlassSurface>
</template>

<style scoped>
.insights {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  overflow: hidden;
}

.insights__card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  padding: 16px 18px;
  border-left: 1px solid var(--dg-card-border);
}

.insights__card:first-child { border-left: 0; }

.insights__label {
  overflow: hidden;
  color: var(--dg-text-tertiary);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.insights__value {
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 17px;
  font-weight: 630;
  letter-spacing: -0.015em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.insights__detail {
  overflow: hidden;
  color: var(--dg-text-muted);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 900px) {
  .insights { grid-template-columns: repeat(2, minmax(0, 1fr)); }

  .insights__card { border-top: 1px solid var(--dg-card-border); }
  .insights__card:nth-child(odd) { border-left: 0; }
  .insights__card:nth-child(-n + 2) { border-top: 0; }
  .insights__card:last-child { grid-column: 1 / -1; }
}
</style>
