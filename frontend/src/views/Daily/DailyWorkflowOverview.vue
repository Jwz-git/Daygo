<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { DailyPresentation, DailyWorkflowCell } from '@/stores/daily'

const props = defineProps<{
  presentation: DailyPresentation
  timeZone: string
}>()

const { locale, t } = useI18n()

const gridStyle = computed(() => ({
  minWidth: `${Math.max(680, props.presentation.slotCount * 18)}px`,
}))

const cellGridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${props.presentation.slotCount}, minmax(13px, 1fr))`,
}))

function cellStyle(cell: DailyWorkflowCell, color: string) {
  if (cell.occupancy <= 0) return undefined
  return {
    '--daily-cell-color': color,
    '--daily-cell-strength': `${Math.round(24 + cell.occupancy * 66)}%`,
  }
}

function formatTime(timestamp: number): string {
  return new Intl.DateTimeFormat(locale.value, {
    hour: 'numeric',
    timeZone: props.timeZone,
  }).format(new Date(timestamp * 1000))
}

function duration(minutes: number): string {
  if (minutes < 60) return t('daily.duration.minutes', { count: minutes })
  const hours = Math.floor(minutes / 60)
  const remainder = minutes % 60
  return remainder === 0
    ? t('daily.duration.hours', { count: hours })
    : t('daily.duration.hoursMinutes', { hours, minutes: remainder })
}
</script>

<template>
  <section class="daily-section" aria-labelledby="daily-workflow-title">
    <header class="section-heading">
      <div>
        <h2 id="daily-workflow-title">{{ t('daily.workflow.title') }}</h2>
        <p>{{ t('daily.workflow.description') }}</p>
      </div>
      <span class="slot-note">{{ t('daily.workflow.slotNote') }}</span>
    </header>

    <div class="workflow-card dg-card">
      <div v-if="presentation.rows.length === 0" class="workflow-empty">
        {{ t('daily.workflow.empty') }}
      </div>

      <div v-else class="workflow-scroll">
        <div class="workflow-grid" :style="gridStyle">
          <div class="workflow-axis-label" aria-hidden="true"></div>
          <div class="workflow-axis" aria-hidden="true">
            <span
              v-for="(tick, index) in presentation.ticks"
              :key="tick.timestamp"
              :class="{
                'is-first': index === 0,
                'is-last': index === presentation.ticks.length - 1,
              }"
              :style="{ left: `${tick.position * 100}%` }"
            >
              {{ formatTime(tick.timestamp) }}
            </span>
          </div>

          <template v-for="row in presentation.rows" :key="row.id">
            <div class="workflow-label">
              <i :style="{ background: row.colorHex }" aria-hidden="true"></i>
              <span>{{ row.name }}</span>
            </div>
            <div class="workflow-cells" :style="cellGridStyle">
              <span
                v-for="(cell, index) in row.cells"
                :key="index"
                class="workflow-cell"
                :class="{ 'is-occupied': cell.occupancy > 0, 'has-distraction': cell.hasDistraction }"
                :style="cellStyle(cell, row.colorHex)"
                :title="cell.title ?? undefined"
                aria-hidden="true"
              ></span>
            </div>
          </template>
        </div>
      </div>

      <div v-if="presentation.rows.some((row) => row.minutes > 0)" class="workflow-totals">
        <span class="workflow-totals__title">{{ t('daily.workflow.total') }}</span>
        <span
          v-for="row in presentation.rows.filter((entry) => entry.minutes > 0)"
          :key="row.id"
          class="workflow-total"
        >
          <i :style="{ background: row.colorHex }" aria-hidden="true"></i>
          <span>{{ row.name }}</span>
          <strong>{{ duration(row.minutes) }}</strong>
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.daily-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}

.section-heading h2 {
  color: var(--dg-text-primary);
  font-size: 18px;
  font-weight: 650;
  line-height: 1.25;
}

.section-heading p,
.slot-note {
  color: var(--dg-text-tertiary);
  font-size: 12px;
}

.section-heading p { margin-top: 3px; }
.slot-note { flex: none; padding-bottom: 1px; }

.workflow-card { overflow: hidden; }

.workflow-scroll {
  padding: 19px 20px 15px;
  overflow-x: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--dg-hover-fill-strong) transparent;
}

.workflow-grid {
  display: grid;
  grid-template-columns: 112px minmax(0, 1fr);
  gap: 5px 14px;
  align-items: center;
}

.workflow-axis-label { height: 22px; }

.workflow-axis {
  position: relative;
  height: 22px;
  border-bottom: 1px solid var(--dg-daily-grid-line);
}

.workflow-axis span {
  position: absolute;
  bottom: 6px;
  color: var(--dg-text-muted);
  font-size: 10px;
  line-height: 1;
  white-space: nowrap;
  transform: translateX(-50%);
}

.workflow-axis span.is-first { transform: none; }
.workflow-axis span.is-last { transform: translateX(-100%); }

.workflow-label {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 7px;
  min-width: 0;
  color: var(--dg-text-secondary);
  font-size: 12px;
  text-align: right;
}

.workflow-label i,
.workflow-total i {
  flex: none;
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.workflow-label span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workflow-cells {
  display: grid;
  gap: 3px;
}

.workflow-cell {
  position: relative;
  height: 17px;
  border: 1px solid var(--dg-daily-cell-border);
  border-radius: 3px;
  background: var(--dg-daily-cell-empty);
}

.workflow-cell.is-occupied {
  border-color: transparent;
  background: color-mix(
    in srgb,
    var(--daily-cell-color) var(--daily-cell-strength),
    var(--dg-daily-cell-empty)
  );
}

.workflow-cell.has-distraction::after {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--dg-daily-distraction);
  content: '';
}

.workflow-totals {
  display: flex;
  align-items: center;
  gap: 13px;
  min-height: 47px;
  padding: 10px 20px;
  overflow-x: auto;
  border-top: 1px solid var(--dg-card-border);
  background: var(--dg-daily-footer-fill);
  white-space: nowrap;
  scrollbar-width: none;
}

.workflow-totals::-webkit-scrollbar { display: none; }
.workflow-totals__title { color: var(--dg-text-muted); font-size: 11px; }

.workflow-total {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--dg-text-secondary);
  font-size: 11px;
}

.workflow-total strong {
  color: var(--dg-text-primary);
  font-weight: 620;
}

.workflow-empty {
  display: grid;
  min-height: 170px;
  padding: 24px;
  color: var(--dg-text-tertiary);
  font-size: 13px;
  place-items: center;
}

@media (max-width: 720px) {
  .slot-note { display: none; }
  .workflow-scroll { padding-right: 16px; padding-left: 16px; }
}
</style>
