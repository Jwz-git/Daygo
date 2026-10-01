<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklyPresentation } from '@/stores/weeklyPresentation'
import { percentageLabel } from '@/stores/weeklyPresentation'

const props = defineProps<{ presentation: WeeklyPresentation }>()
const { t } = useI18n()

const duration = useDurationFormat()

const focusPercent = computed(() => percentageLabel(props.presentation.focusShare))
const ringStyle = computed(() => ({
  '--weekly-focus-angle': `${props.presentation.focusShare * 360}deg`,
}))
</script>

<template>
  <LiquidGlassSurface intensity="air" as="section" class="overview" :aria-label="t('weekly.overview.title')">
    <div class="overview__header">
      <div>
        <p class="overview__eyebrow">{{ t('weekly.overview.eyebrow') }}</p>
        <h2>{{ t('weekly.overview.title') }}</h2>
      </div>
      <p>{{ t('weekly.overview.description') }}</p>
    </div>

    <div class="overview__body">
      <div
        class="focus-ring"
        :style="ringStyle"
        role="img"
        :aria-label="t('weekly.overview.focusAria', { value: focusPercent })"
      >
        <div class="focus-ring__center">
          <strong>{{ focusPercent }}</strong>
          <span>{{ t('weekly.metric.focusRate') }}</span>
        </div>
      </div>

      <dl class="metrics">
        <div>
          <dt>{{ t('weekly.metric.tracked') }}</dt>
          <dd>{{ duration(presentation.trackedMinutes) }}</dd>
        </div>
        <div>
          <dt>{{ t('weekly.metric.focused') }}</dt>
          <dd>{{ duration(presentation.focusMinutes) }}</dd>
        </div>
        <div>
          <dt>{{ t('weekly.metric.other') }}</dt>
          <dd>{{ duration(presentation.otherMinutes) }}</dd>
        </div>
      </dl>
    </div>
  </LiquidGlassSurface>
</template>

<style scoped>
.overview { overflow: hidden; }

.overview__header {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 20px 24px 16px;
  border-bottom: 0.5px solid var(--dg-separator);
}

.overview__eyebrow {
  display: inline-block;
  color: var(--dg-weekly-tag-text);
  font-size: var(--dg-text-callout);
  font-weight: 600;
}

.overview h2 {
  margin-top: 2px;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-display);
  font-size: var(--dg-text-title2);
  font-weight: 650;
  letter-spacing: 0;
}

.overview__header > p {
  color: var(--dg-text-tertiary);
  font-size: 12px;
  line-height: 1.55;
}

.overview__body {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  align-items: center;
  min-height: 244px;
  padding: 26px;
}

.focus-ring {
  display: grid;
  width: 156px;
  height: 156px;
  margin: 0 auto;
  border-radius: 50%;
  background: conic-gradient(
    var(--dg-weekly-focus) 0 var(--weekly-focus-angle),
    var(--dg-weekly-ring-track) var(--weekly-focus-angle) 360deg
  );
  /* The outer glow is the ring's light spilling onto the glass beneath it. */
  box-shadow: none;
  place-items: center;
}

.focus-ring__center {
  display: grid;
  width: 116px;
  height: 116px;
  border: 0.5px solid var(--dg-separator);
  border-radius: 50%;
  background: var(--dg-weekly-ring-center);
  place-content: center;
  text-align: center;
}

.focus-ring strong {
  color: var(--dg-text-primary);
  font-family: var(--dg-font-display);
  font-size: 38px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1;
  letter-spacing: 0;
}

.focus-ring span {
  margin-top: 1px;
  color: var(--dg-text-tertiary);
  font-size: 11px;
}

.metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 0.5px solid var(--dg-separator);
  border-radius: 8px;
  background: var(--dg-weekly-summary-fill);
}

.metrics div {
  min-width: 0;
  padding: 22px 18px;
}

.metrics div + div { border-left: 0.5px solid var(--dg-separator); }

.metrics dt {
  color: var(--dg-text-tertiary);
  font-size: 11px;
  font-weight: 600;
}

.metrics dd {
  margin-top: 6px;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-display);
  font-size: 23px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0;
  white-space: nowrap;
}

@media (max-width: 760px) {
  .overview__body { grid-template-columns: minmax(0, 1fr); gap: 26px; }
  .metrics { width: 100%; }
}

@media (max-width: 520px) {
  .metrics { grid-template-columns: minmax(0, 1fr); }
  .metrics div + div { border-top: 0.5px solid var(--dg-separator); border-left: 0; }
}
</style>
