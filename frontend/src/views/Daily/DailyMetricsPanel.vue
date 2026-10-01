<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { useDurationFormat } from '@/lib/duration'
import type { DailyMetrics } from '@/stores/daily'

const props = defineProps<{ metrics: DailyMetrics }>()
const { t } = useI18n()

const duration = useDurationFormat()

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
  <LiquidGlassSurface intensity="air" as="section" class="metrics" :aria-label="t('daily.stats.title')">
    <div v-for="item in items" :key="item.key" class="metric">
      <span>{{ item.label }}</span>
      <strong>{{ item.value }}</strong>
    </div>
  </LiquidGlassSurface>
</template>

<style scoped>
/* Surface comes from .dg-card; only the metric grid lives here. */
.metrics {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  overflow: hidden;
}

.metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  padding: 14px 16px;
  border-left: 0.5px solid var(--dg-separator);
}

.metric:first-child { border-left: 0; }
.metric span { color: var(--dg-text-secondary); font-size: var(--dg-text-footnote); }
.metric strong {
  color: var(--dg-text-primary);
  font-family: var(--dg-font-display);
  font-size: 20px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: var(--dg-font-display-tracking);
}

@media (max-width: 840px) {
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .metric { border-top: 1px solid var(--dg-card-border); }
  .metric:nth-child(odd) { border-left: 0; }
  .metric:nth-child(-n + 2) { border-top: 0; }
  .metric:last-child { grid-column: 1 / -1; }
}
</style>
