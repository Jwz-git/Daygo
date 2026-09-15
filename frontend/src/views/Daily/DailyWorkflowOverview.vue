<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { DailyPresentation, DailyWorkflowCell, DailyWorkflowRow } from '@/stores/daily'
import { SLOT_SECONDS } from '@/stores/daily'
import { useDurationFormat } from '@/lib/duration'
import { categoryLabel } from '@/lib/categoryLabel'
import { safeTimeZone } from '@/lib/timeZone'

const props = defineProps<{
  presentation: DailyPresentation
  timeZone: string
}>()

const { locale, t } = useI18n()

const gridStyle = computed(() => ({
  minWidth: `${Math.max(680, props.presentation.slotCount * 18)}px`,
}))

const cellGridStyle = computed(() => ({
  // Fixed square cells: a fluid 1fr track squeezed cells into non-squares.
  gridTemplateColumns: `repeat(${props.presentation.slotCount}, 15px)`,
}))

function cellStyle(cell: DailyWorkflowCell, color: string) {
  if (cell.occupancy <= 0) return undefined
  return {
    '--daily-cell-color': color,
    '--daily-cell-strength': `${Math.round(24 + cell.occupancy * 66)}%`,
  }
}

/*
 * Hover tooltip (GitHub-contributions style): the slot's minutes in the row's
 * colour plus the card title covering it. One open tooltip at a time.
 */
const hoveredCell = ref<{ rowId: string; index: number } | null>(null)

function tooltipOf(row: DailyWorkflowRow, index: number): { minutes: string; title: string } | null {
  const state = hoveredCell.value
  if (state === null || state.rowId !== row.id || state.index !== index) return null
  const cell = row.cells[index]
  if (cell === undefined) return null
  return {
    minutes: duration(Math.round(cell.occupancy * (SLOT_SECONDS / 60))),
    title: cell.title ?? categoryLabel(row.name, t),
  }
}

function formatTime(timestamp: number): string {
  return new Intl.DateTimeFormat(locale.value, {
    hour: 'numeric',
    timeZone: safeTimeZone(props.timeZone),
  }).format(new Date(timestamp * 1000))
}

const duration = useDurationFormat()
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
              <span>{{ categoryLabel(row.name, t) }}</span>
            </div>
            <div class="workflow-cells" :style="cellGridStyle">
              <span
                v-for="(cell, index) in row.cells"
                :key="index"
                class="workflow-cell"
                :class="{ 'is-occupied': cell.occupancy > 0, 'has-distraction': cell.hasDistraction }"
                :style="cellStyle(cell, row.colorHex)"
                @mouseenter="hoveredCell = { rowId: row.id, index }"
                @mouseleave="hoveredCell = null"
              >
                <span
                  v-if="tooltipOf(row, index) !== null"
                  class="workflow-tip"
                  role="status"
                >
                  <strong :style="{ color: row.colorHex }">{{ tooltipOf(row, index)!.minutes }}</strong>
                  <span>{{ tooltipOf(row, index)!.title }}</span>
                </span>
              </span>
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
          <span>{{ categoryLabel(row.name, t) }}</span>
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
  width: 15px;
  height: 15px;
  border: none;
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

/* Hover tooltip floating above the cell (GitHub style). */
.workflow-tip {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 50%;
  z-index: 20;
  display: grid;
  gap: 2px;
  width: max-content;
  max-width: 260px;
  padding: 8px 11px;
  border-radius: 8px;
  background: var(--dg-popover-fill, var(--dg-surface));
  box-shadow: var(--dg-shadow-lg);
  text-align: left;
  transform: translateX(-50%);
  pointer-events: none;
}

.workflow-tip strong {
  font-size: 12px;
  font-weight: 650;
}

.workflow-tip span {
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 12px;
  line-height: 1.4;
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
}

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
