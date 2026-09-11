<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { DailyMetrics } from '@/stores/daily'

const props = defineProps<{ metrics: DailyMetrics }>()
const { t } = useI18n()

function duration(minutes: number): string {
  if (minutes < 60) return t('daily.duration.minutes', { count: minutes })
  const hours = Math.floor(minutes / 60)
  const remainder = minutes % 60
  return remainder === 0
    ? t('daily.duration.hours', { count: hours })
    : t('daily.duration.hoursMinutes', { hours, minutes: remainder })
}

const items = computed(() => [
  {
    key: 'context',
    label: t('daily.stats.contextSwitched'),
    value: t('daily.stats.times', { count: props.metrics.contextSwitches }),
  },
  {
    key: 'interruptions',
    label: t('daily.stats.interrupted'),
    value: t('daily.stats.times', { count: props.metrics.interruptions }),
  },
  { key: 'focused', label: t('daily.stats.focusedFor'), value: duration(props.metrics.focusedMinutes) },
  {
    key: 'distracted',
    label: t('daily.stats.distractedFor'),
    value: duration(props.metrics.distractedMinutes),
  },
  {
    key: 'transition',
    label: t('daily.stats.transitioning'),
    value: duration(props.metrics.transitioningMinutes),
  },
])
</script>

<template>
  <section class="metrics" :aria-label="t('daily.stats.title')">
    <div v-for="item in items" :key="item.key" class="metric">
      <span>{{ item.label }}</span>
      <strong>{{ item.value }}</strong>
    </div>
  </section>
</template>

<style scoped>
.metrics {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--dg-card-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-card-fill);
  box-shadow: var(--dg-card-shadow);
}

.metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  padding: 14px 16px;
  border-left: 1px solid var(--dg-card-border);
}

.metric:first-child { border-left: 0; }
.metric span { color: var(--dg-text-tertiary); font-size: 11px; }
.metric strong { color: var(--dg-text-primary); font-size: 17px; font-weight: 630; }

@media (max-width: 840px) {
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .metric { border-top: 1px solid var(--dg-card-border); }
  .metric:nth-child(odd) { border-left: 0; }
  .metric:nth-child(-n + 2) { border-top: 0; }
  .metric:last-child { grid-column: 1 / -1; }
}
</style>
