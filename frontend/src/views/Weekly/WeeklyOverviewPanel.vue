<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { WeeklyPresentation } from '@/stores/weeklyPresentation'
import { percentageLabel } from '@/stores/weeklyPresentation'

const props = defineProps<{ presentation: WeeklyPresentation }>()
const { t } = useI18n()

function duration(minutes: number): string {
  const rounded = Math.max(0, Math.round(minutes))
  if (rounded < 60) return t('weekly.duration.minutes', { count: rounded })
  const hours = Math.floor(rounded / 60)
  const remainder = rounded % 60
  return remainder === 0
    ? t('weekly.duration.hours', { count: hours })
    : t('weekly.duration.hoursMinutes', { hours, minutes: remainder })
}

const focusPercent = computed(() => percentageLabel(props.presentation.focusShare))
const ringStyle = computed(() => ({
  '--weekly-focus-angle': `${props.presentation.focusShare * 360}deg`,
}))
</script>

<template>
  <section class="overview dg-card" :aria-label="t('weekly.overview.title')">
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
  </section>
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
  color: var(--dg-accent-text);
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.02em;
}

.overview h2 {
  margin-top: 4px;
  color: var(--dg-text-primary);
  font-size: 18px;
  font-weight: 650;
  letter-spacing: -0.012em;
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
  box-shadow: inset 0 0 0 1px var(--dg-card-border);
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
  font-size: 27px;
  font-weight: 620;
  letter-spacing: -0.03em;
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
  margin-top: 7px;
  color: var(--dg-text-primary);
  font-size: 17px;
  font-weight: 620;
  letter-spacing: -0.015em;
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
