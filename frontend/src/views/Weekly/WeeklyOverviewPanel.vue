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
  display: grid;
  grid-template-columns: minmax(220px, 0.8fr) minmax(280px, 1.2fr);
  align-items: end;
  gap: 32px;
  padding: 24px 26px 20px;
  border-bottom: 1px solid var(--dg-card-border);
}

.overview__eyebrow {
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

.overview h2 {
  margin-top: 7px;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-serif);
  font-size: 24px;
  font-weight: 400;
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
  box-shadow: inset 0 0 0 1px var(--dg-card-border), 0 0 36px var(--dg-weekly-ring-glow);
  place-items: center;
}

.focus-ring__center {
  display: grid;
  width: 116px;
  height: 116px;
  border: 1px solid var(--dg-card-border);
  border-radius: 50%;
  background: var(--dg-weekly-ring-center);
  place-content: center;
  text-align: center;
}

.focus-ring strong {
  color: var(--dg-text-primary);
  font-family: var(--dg-font-serif);
  font-size: 38px;
  font-weight: 400;
  line-height: 1;
  letter-spacing: 0;
}

.focus-ring span {
  margin-top: 1px;
  color: var(--dg-text-muted);
  font-size: 10px;
}

.metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid var(--dg-card-border);
  border-radius: 8px;
  background: var(--dg-weekly-summary-fill);
}

.metrics div {
  min-width: 0;
  padding: 22px 18px;
}

.metrics div + div { border-left: 1px solid var(--dg-card-border); }

.metrics dt {
  color: var(--dg-text-muted);
  font-size: 10px;
  font-weight: 600;
}

.metrics dd {
  margin-top: 6px;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-serif);
  font-size: 23px;
  font-weight: 400;
  letter-spacing: 0;
  white-space: nowrap;
}

@media (max-width: 760px) {
  .overview__header { grid-template-columns: minmax(0, 1fr); gap: 8px; }
  .overview__body { grid-template-columns: minmax(0, 1fr); gap: 26px; }
  .metrics { width: 100%; }
}

@media (max-width: 520px) {
  .metrics { grid-template-columns: minmax(0, 1fr); }
  .metrics div + div { border-top: 1px solid var(--dg-card-border); border-left: 0; }
}
</style>
