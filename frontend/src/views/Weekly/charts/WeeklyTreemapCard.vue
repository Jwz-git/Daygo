<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { squarify, type Rect } from '@/lib/chartLayout'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklyTreemapApp, WeeklyTreemapCategory } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
import WeeklyChartTooltip from './WeeklyChartTooltip.vue'
import { useChartPointer } from './useChartPointer'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'

/*
 * "Most used per category" (Dayflow WeeklyTreemapSection): categories as
 * squarified shells tinted with their colour (25% fill, 75% border), apps as
 * squarified tiles inside (42% fill), each with its time and the change
 * against last week. Laid out in a fixed 800×400 design space and placed by
 * percentage so the card scales with the page.
 */
const props = defineProps<{ categories: WeeklyTreemapCategory[]; days: string[] }>()

const { t } = useI18n()
const formatDuration = useDurationFormat()
const labels = useWeeklyChartLabels(() => props.days)

const WIDTH = 800
const HEIGHT = 400
const CATEGORY_GAP = 6
const TILE_GAP = 4
const HEADER = 26
const PADDING = 6

function percentStyle(rect: Rect): Record<string, string> {
  return {
    left: `${(rect.x / WIDTH) * 100}%`,
    top: `${(rect.y / HEIGHT) * 100}%`,
    width: `${(rect.width / WIDTH) * 100}%`,
    height: `${(rect.height / HEIGHT) * 100}%`,
  }
}

// Dayflow's leaf rules, judged on the rendered size so a narrow window drops
// detail before text overflows: a typography tier (WeeklyTreemapLeafTypography)
// and a presentation mode (WeeklyTreemapLeafPresentationMode). Sizes are one
// step larger than Dayflow's 20 / 16 / 13 serif names for this window.
type TileMode = 'full' | 'compact' | 'labelOnly'

interface TileType {
  name: number
  detail: number
  delta: number
  gap: number
  /** Vertical padding (Dayflow's per-tier padding). */
  padding: number
  /** Horizontal padding, wider than Dayflow's so names keep clear of the tile edges. */
  paddingX: number
}

const TILE_TYPES: Record<'large' | 'medium' | 'compact', TileType> = {
  large: { name: 22, detail: 12, delta: 10.5, gap: 4, padding: 12, paddingX: 22 },
  medium: { name: 18, detail: 11.5, delta: 10, gap: 3, padding: 10, paddingX: 16 },
  compact: { name: 14.5, detail: 10.5, delta: 9.5, gap: 2, padding: 6, paddingX: 10 },
}

interface TilePlacement {
  app: WeeklyTreemapApp
  rect: Rect
  mode: TileMode
  type: TileType
  id: string
  color: string
  category: WeeklyTreemapCategory
}

// Rendered pixels per design unit, tracked from the container's width.
// Updates wait a frame so a layout pass never loops back into the observer.
const scale = ref(1)
let observer: ResizeObserver | null = null
let frame = 0
function observe(element: HTMLElement | null): void {
  observer?.disconnect()
  if (element === null || typeof ResizeObserver === 'undefined') return
  observer = new ResizeObserver(([entry]) => {
    const next = (entry?.contentRect.width ?? 0) / WIDTH || 1
    cancelAnimationFrame(frame)
    frame = requestAnimationFrame(() => {
      if (Math.abs(next - scale.value) > 0.001) scale.value = next
    })
  })
  observer.observe(element)
}
onBeforeUnmount(() => {
  observer?.disconnect()
  cancelAnimationFrame(frame)
})

function tileType(rect: Rect): TileType {
  const width = rect.width * scale.value
  const height = rect.height * scale.value
  if (width >= 160 && height >= 110) return TILE_TYPES.large
  if (width >= 90 && height >= 54) return TILE_TYPES.medium
  return TILE_TYPES.compact
}

// A mode is used only when its lines fit the tile at this tier's sizes:
// name row, time, and in full mode the change badge.
function tileMode(rect: Rect, app: WeeklyTreemapApp, type: TileType): TileMode {
  const width = rect.width * scale.value
  const height = rect.height * scale.value
  const nameRow = (size: number): number => Math.max(13, size * 1.1) * 1.15
  const time = type.detail * 1.35
  const badge = changeLabel(app.changeMinutes) !== null ? type.gap + type.delta * 1.3 + 2 : 0
  const fullHeight = type.padding * 2 + nameRow(type.name) + type.gap + time + badge
  const compactHeight = type.padding * 2 + nameRow(Math.max(type.name - 2, 12)) + type.gap + time
  if (width >= 96 && height >= fullHeight) return 'full'
  if (width >= 64 && height >= compactHeight) return 'compact'
  return 'labelOnly'
}

// Name size per mode, as Dayflow shrinks compact and label-only tiles.
function nameSize(tile: TilePlacement): number {
  if (tile.mode === 'full') return tile.type.name
  if (tile.mode === 'compact') return Math.max(tile.type.name - 2, 12)
  return Math.max(tile.type.name - 3, 11)
}

function detailSize(tile: TilePlacement): number {
  return tile.mode === 'full' ? tile.type.detail : Math.max(tile.type.detail - 1, 11)
}

function narrowShell(rect: Rect): boolean {
  return rect.width * scale.value < 170
}

const layout = computed(() =>
  squarify(props.categories, (category) => category.minutes, { x: 0, y: 0, width: WIDTH, height: HEIGHT }, CATEGORY_GAP)
    .map(({ item: category, rect }) => {
      const inner: Rect = {
        x: rect.x + PADDING,
        y: rect.y + HEADER,
        width: Math.max(0, rect.width - PADDING * 2),
        height: Math.max(0, rect.height - HEADER - PADDING),
      }
      const tiles: TilePlacement[] = squarify(category.apps, (app) => app.minutes, inner, TILE_GAP)
        .map(({ item: app, rect: tile }) => ({
          app,
          rect: tile,
          mode: tileMode(tile, app, tileType(tile)),
          type: tileType(tile),
          id: `${category.name}\u0000${app.key}`,
          color: category.colorHex,
          category,
        }))
      return { category, rect, tiles }
    }),
)
const tiles = computed(() => layout.value.flatMap((entry) => entry.tiles))

// Dayflow's treemap hover card: the hovered tile lifts, the rest recede.
const pointer = useChartPointer<TilePlacement>()
watch(pointer.container, observe)
const hoveredId = computed(() => pointer.hovered.value?.id ?? null)

function shareOfCategory(tile: TilePlacement): string {
  return `${Math.round((tile.app.minutes / Math.max(1, tile.category.minutes)) * 100)}%`
}

// Dayflow's weeklyTreemapDurationString: "6hr 4m", "6hr" or "46m".
function compactDuration(minutes: number): string {
  const total = Math.max(0, Math.round(minutes))
  const hours = Math.floor(total / 60)
  const rest = total % 60
  if (hours > 0 && rest > 0) return t('weekly.charts.treemap.hoursMinutes', { hours, minutes: rest })
  if (hours > 0) return t('weekly.charts.treemap.hours', { hours })
  return t('weekly.charts.treemap.minutes', { minutes: rest })
}

// Dayflow's change badge always counts minutes: "+ 152m", "- 30m".
function changeBadge(minutes: number | null): string | null {
  if (minutes === null || minutes === 0) return null
  return `${minutes > 0 ? '+' : '-'} ${t('weekly.charts.treemap.minutes', { minutes: Math.abs(Math.round(minutes)) })}`
}

function changeLabel(minutes: number | null): string | null {
  if (minutes === null || minutes === 0) return null
  return `${minutes > 0 ? '+' : '−'} ${formatDuration(Math.abs(minutes))}`
}
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.treemap.title')">
    <p v-if="categories.length === 0" class="tm__empty">{{ t('weekly.charts.treemap.empty') }}</p>
    <div
      v-else
      :ref="(el) => { pointer.container.value = el as HTMLElement | null }"
      class="tm"
      :class="{ 'has-hover': hoveredId !== null }"
      role="img"
      :aria-label="t('weekly.charts.treemap.aria')"
    >
      <section
        v-for="entry in layout"
        :key="entry.category.name"
        class="tm__shell"
        :style="{ ...percentStyle(entry.rect), '--tm-color': entry.category.colorHex }"
      >
        <header class="tm__header">
          <span>{{ labels.category(entry.category.name) }}</span>
          <b v-if="!narrowShell(entry.rect)">{{ compactDuration(entry.category.minutes) }}</b>
        </header>
      </section>
      <div
        v-for="tile in tiles"
        :key="tile.id"
        class="tm__tile"
        :class="{ 'is-hovered': hoveredId === tile.id }"
        :style="{
          ...percentStyle(tile.rect),
          '--tm-color': tile.color,
          '--tm-name': `${nameSize(tile)}px`,
          '--tm-detail': `${detailSize(tile)}px`,
          '--tm-delta': `${tile.type.delta}px`,
          gap: `${tile.type.gap}px`,
          padding: `${tile.type.padding}px ${tile.type.paddingX}px`,
        }"
        @pointermove="pointer.move($event, tile)"
        @pointerleave="pointer.leave"
      >
        <span class="tm__name-row">
          <AppSiteIcon
            v-if="tile.app.sites.length > 0"
            :sites="tile.app.sites"
            :size="Math.max(13, Math.round(nameSize(tile) * 1.1))"
            :accent="tile.color"
          />
          <span class="tm__name">{{ labels.app(tile.app.key, tile.app.name) }}</span>
        </span>
        <span v-if="tile.mode !== 'labelOnly'" class="tm__time">{{ compactDuration(tile.app.minutes) }}</span>
        <span
          v-if="tile.mode === 'full' && changeLabel(tile.app.changeMinutes)"
          class="tm__change"
          :class="(tile.app.changeMinutes ?? 0) > 0 ? 'is-up' : 'is-down'"
        >{{ changeBadge(tile.app.changeMinutes) }}</span>
      </div>
      <WeeklyChartTooltip :visible="pointer.hovered.value !== null" :x="pointer.x.value" :y="pointer.y.value">
        <template v-if="pointer.hovered.value">
          <b>{{ labels.app(pointer.hovered.value.app.key, pointer.hovered.value.app.name) }}</b>
          <span>{{ labels.category(pointer.hovered.value.category.name) }} · {{ formatDuration(pointer.hovered.value.app.minutes) }}</span>
          <span>{{ t('weekly.charts.tooltip.share', { name: labels.category(pointer.hovered.value.category.name), value: shareOfCategory(pointer.hovered.value) }) }}</span>
          <span>{{
            pointer.hovered.value.app.changeMinutes === null
              ? t('weekly.charts.tooltip.newThisWeek')
              : t('weekly.charts.tooltip.change', { value: changeLabel(pointer.hovered.value.app.changeMinutes) ?? '0' })
          }}</span>
        </template>
      </WeeklyChartTooltip>
    </div>
  </WeeklyChartCard>
</template>

<style scoped>
.tm {
  position: relative;
  width: 100%;
  aspect-ratio: 2 / 1;
  min-height: 280px;
}

.tm__empty {
  margin: 24px 0;
  color: var(--dg-wk-text-muted);
  font-size: 12px;
}

.tm__shell {
  position: absolute;
  box-sizing: border-box;
  border: 1px solid color-mix(in srgb, var(--tm-color) 75%, transparent);
  border-radius: 4px;
  background: color-mix(in srgb, var(--tm-color) 25%, transparent);
}

.tm__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px 0;
  overflow: hidden;
  color: #4a4a4a;
  font-size: 13.5px;
  font-weight: 650;
  white-space: nowrap;
}

:root[data-dg-appearance='dark'] .tm__header { color: #dfdfdf; }

.tm__header span {
  overflow: hidden;
  text-overflow: ellipsis;
}

.tm__header b {
  flex: none;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.tm__tile {
  position: absolute;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px;
  overflow: hidden;
  border: 1px solid var(--tm-color);
  border-radius: 4px;
  background: color-mix(in srgb, var(--tm-color) 42%, transparent);
  color: var(--dg-wk-text);
  text-align: center;
  cursor: default;
  transition:
    opacity 220ms cubic-bezier(0.22, 1, 0.36, 1),
    transform 260ms cubic-bezier(0.22, 1, 0.36, 1),
    box-shadow 260ms cubic-bezier(0.22, 1, 0.36, 1),
    background-color 220ms ease-out;
}

/* Other tiles recede by their fill only; their text stays readable. */
.tm.has-hover .tm__tile:not(.is-hovered) {
  border-color: color-mix(in srgb, var(--tm-color) 45%, transparent);
  background: color-mix(in srgb, var(--tm-color) 20%, transparent);
}

.tm__tile > * {
  transition: opacity 220ms cubic-bezier(0.22, 1, 0.36, 1);
}

.tm.has-hover .tm__tile:not(.is-hovered) > * {
  opacity: 0.6;
}

.tm__tile.is-hovered {
  z-index: 2;
  background: color-mix(in srgb, var(--tm-color) 58%, transparent);
  box-shadow: 0 10px 24px -12px color-mix(in srgb, var(--tm-color) 80%, transparent);
  transform: scale(1.025);
}

@media (prefers-reduced-motion: reduce) {
  .tm__tile,
  .tm__tile > * { transition: none; }
  .tm__tile.is-hovered { transform: none; }
}

.tm__name-row {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  max-width: 100%;
  min-width: 0;
}

.tm__name-row > :not(.tm__name) {
  flex: none;
}

/*
 * Dayflow keeps the Instrument Serif name. The face ships one weight, so it
 * is thickened with a hairline stroke rather than a synthesised bold.
 */
.tm__name {
  min-width: 0;
  overflow: hidden;
  color: var(--dg-wk-text);
  font-family: var(--dg-font-serif);
  font-size: var(--tm-name);
  font-weight: 400;
  -webkit-text-stroke: 0.45px currentColor;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tm__time,
.tm__change {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tm__time {
  color: var(--dg-wk-text);
  font-size: var(--tm-detail);
  font-weight: 400;
  font-variant-numeric: tabular-nums;
}

/* Dayflow's change badge: monospaced figures on a tinted pill. */
.tm__change {
  padding: 1px 5px;
  border-radius: 3px;
  font-family: ui-monospace, 'SF Mono', Menlo, monospace;
  font-size: var(--tm-delta);
  font-weight: 400;
}

.tm__change.is-up { background: #d1eae4; color: #089041; }
.tm__change.is-down { background: #fae0d8; color: #e25922; }

:root[data-dg-appearance='dark'] .tm__change.is-up { background: #58927f; color: #e0fbee; }
:root[data-dg-appearance='dark'] .tm__change.is-down { background: #ca6d59; color: #fce3e1; }
</style>
