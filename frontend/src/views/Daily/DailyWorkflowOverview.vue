<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type {
  DailyPresentation,
  DailyWorkflowCell,
  DailyWorkflowDistractionMarker,
  DailyWorkflowRow,
} from '@/stores/daily'
import { isDistractionCategoryKey, SLOT_SECONDS } from '@/stores/daily'
import { useDurationFormat } from '@/lib/duration'
import { categoryLabel } from '@/lib/categoryLabel'
import { safeTimeZone } from '@/lib/timeZone'

const props = defineProps<{
  presentation: DailyPresentation
  timeZone: string
}>()

const { locale, t } = useI18n()

const gridStyle = computed(() => ({
  minWidth: `${Math.max(680, props.presentation.slotCount * 20)}px`,
}))

// Dayflow baseline: 18px cell + 2px gap. Cells, axis and the distraction
// track all share this exact width so hour ticks land on cell boundaries.
const gridWidth = computed(() => props.presentation.slotCount * 20 - 2)

const cellGridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${props.presentation.slotCount}, 18px)`,
  width: `${gridWidth.value}px`,
}))

function cellStyle(cell: DailyWorkflowCell, color: string) {
  if (cell.occupancy <= 0) return undefined
  // Partial occupancy stays dimmer; full occupancy reaches full intensity.
  return { '--daily-cell-color': color, '--daily-cell-alpha': `${0.3 + cell.occupancy * 0.7}` }
}

/*
 * One distraction lane replaces the category squares whenever the day has a
 * Distraction category or any embedded distraction markers. It remains as an
 * empty track when there are no markers, so the Distraction row never falls
 * back to a second square grid.
 */
const showDistractionTrack = computed(
  () =>
    props.presentation.hasDistractionCategory ||
    props.presentation.distractionMarkers.length > 0,
)

const displayRows = computed(() => {
  if (showDistractionTrack.value) {
    return props.presentation.rows.filter((row) => !isDistractionCategoryKey(row.name))
  }
  return props.presentation.rows
})

function markerStyle(marker: DailyWorkflowDistractionMarker) {
  const totalSec = props.presentation.windowEndTs - props.presentation.windowStartTs
  if (totalSec <= 0) return { left: '0px', width: '6px' }
  const startFraction = Math.max(0, (marker.startTs - props.presentation.windowStartTs) / totalSec)
  const endFraction = Math.min(1, (marker.endTs - props.presentation.windowStartTs) / totalSec)
  const left = startFraction * gridWidth.value
  const width = Math.max(4, (endFraction - startFraction) * gridWidth.value)
  return {
    left: `${left}px`,
    width: `${width}px`,
  }
}

/*
 * Hover tooltip (GitHub-contributions style): the slot's minutes in the row's
 * colour plus the card title covering it. One open tooltip at a time.
 */
interface TooltipState {
  rowId: string
  index: number
  color: string
  minutes: string
  title: string
  x: number
  y: number
}

/*
 * Dayflow's anti-flicker hover: leaving a cell schedules the hide 80ms out,
 * so gliding across adjacent cells keeps one bubble that slides from cell to
 * cell instead of blinking off and on.
 */
const tooltipData = ref<TooltipState | null>(null)
const tooltipVisible = ref(false)
let hideTimer: number | null = null

function onCellEnter(
  event: MouseEvent,
  row: DailyWorkflowRow,
  index: number,
): void {
  const cell = row.cells[index]
  if (hideTimer !== null) {
    window.clearTimeout(hideTimer)
    hideTimer = null
  }
  // Empty cells carry no bubble (Dayflow only annotates recorded time).
  if (cell === undefined || cell.occupancy <= 0) {
    tooltipVisible.value = false
    return
  }
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  tooltipData.value = {
    rowId: row.id,
    index,
    color: row.colorHex,
    minutes: duration(Math.round(cell.occupancy * (SLOT_SECONDS / 60))),
    title: cell.title ?? categoryLabel(row.name, t),
    x: rect.left + rect.width / 2,
    y: rect.top,
  }
  tooltipVisible.value = true
}

function onCellLeave(): void {
  if (hideTimer !== null) window.clearTimeout(hideTimer)
  hideTimer = window.setTimeout(() => {
    tooltipVisible.value = false
    hideTimer = null
  }, 80)
}

function onDistractionEnter(
  event: MouseEvent,
  marker: DailyWorkflowDistractionMarker,
): void {
  if (hideTimer !== null) {
    window.clearTimeout(hideTimer)
    hideTimer = null
  }
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  tooltipData.value = {
    rowId: marker.id,
    index: -1,
    color: '#FF653B',
    minutes: duration(marker.durationMinutes),
    title: marker.title || t('timeline.category.distraction'),
    x: rect.left + rect.width / 2,
    y: rect.top,
  }
  tooltipVisible.value = true
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
          <div class="workflow-axis" aria-hidden="true" :style="{ width: `${gridWidth}px` }">
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

          <template v-for="row in displayRows" :key="row.id">
            <div class="workflow-label">
              <i :style="{ background: row.colorHex }" aria-hidden="true"></i>
              <span>{{ categoryLabel(row.name, t) }}</span>
            </div>
            <div class="workflow-cells" :style="cellGridStyle">
              <span
                v-for="(cell, index) in row.cells"
                :key="index"
                class="workflow-cell"
                :class="{ 'is-occupied': cell.occupancy > 0, 'has-distraction': !showDistractionTrack && cell.hasDistraction }"
                :style="cellStyle(cell, row.colorHex)"
                @mouseenter="onCellEnter($event, row, index)"
                @mouseleave="onCellLeave"
              ></span>
            </div>
          </template>

          <template v-if="showDistractionTrack">
            <div class="workflow-label workflow-distraction-label">
              <span>{{ t('daily.workflow.distractions') }}</span>
            </div>
            <div class="workflow-distraction-cell">
              <div class="workflow-distraction-track" :style="{ width: `${gridWidth}px` }">
                <span
                  v-for="marker in presentation.distractionMarkers"
                  :key="marker.id"
                  class="workflow-distraction-marker"
                  :style="markerStyle(marker)"
                  @mouseenter="onDistractionEnter($event, marker)"
                  @mouseleave="onCellLeave"
                ></span>
              </div>
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

  <!-- Fixed-position tooltip: escapes every scroll container so nothing
       clips it. Stays mounted through the fade so gliding between adjacent
       cells reads as one bubble moving, not a blink. -->
  <Teleport to="body">
    <div
      v-if="tooltipData !== null"
      class="workflow-tip"
      :class="{ 'is-visible': tooltipVisible }"
      role="status"
      :style="tooltipData === null ? {} : { left: `${tooltipData.x}px`, top: `${tooltipData.y}px` }"
    >
      <template v-if="tooltipData !== null">
        <strong :style="{ color: tooltipData.color }">{{ tooltipData.minutes }}</strong>
        <span>{{ tooltipData.title }}</span>
      </template>
    </div>
  </Teleport>
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
  gap: 2px 13px;
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
  gap: 2px;
}

.workflow-cell {
  position: relative;
  width: 18px;
  height: 18px;
  border: none;
  /* Dayflow corner radius: 2.5px on the 18px square. */
  border-radius: 2.5px;
  background: var(--dg-daily-cell-empty);
}

.workflow-cell.is-occupied {
  background: color-mix(in srgb, var(--daily-cell-color) calc(var(--daily-cell-alpha) * 100%), transparent);
}

/* Dedicated distraction marker row under the category rows. */
.workflow-distraction-track {
  position: relative;
  height: 10px;
  border-radius: 2px;
  background: var(--dg-daily-distraction-track, var(--dg-track-fill));
}

.workflow-distraction-track span {
  position: absolute;
  top: 0;
  height: 10px;
  border-radius: 2px;
  background: #ff653b;
  cursor: pointer;
  transition: filter 0.12s ease;
}

.workflow-distraction-track span:hover {
  filter: brightness(1.15);
}

/* Hover tooltip floating above the cell (GitHub style). */
.workflow-tip {
  position: fixed;
  z-index: 40;
  display: grid;
  gap: 4px;
  width: 200px;
  padding: 8px;
  border: 1px solid var(--dg-timeline-card-border);
  border-radius: 4px;
  background: var(--dg-popover-fill, var(--dg-surface));
  box-shadow: 0 2px 2px rgba(0, 0, 0, 0.12);
  text-align: left;
  transform: translate(-50%, calc(-100% - 6px));
  opacity: 0;
  transition:
    left 130ms var(--dg-ease-glide),
    top 130ms var(--dg-ease-glide),
    opacity 150ms ease;
  pointer-events: none;
}

.workflow-tip.is-visible { opacity: 1; }

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
