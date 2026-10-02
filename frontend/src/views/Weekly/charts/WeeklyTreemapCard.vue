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

// How much a tile shows depends on its rendered size, not its design-space
// size, so a narrow window drops the icon and change before text overflows.
type TileDetail = 'full' | 'medium' | 'compact'

interface TilePlacement {
  app: WeeklyTreemapApp
  rect: Rect
  detail: TileDetail
  id: string
  color: string
  category: WeeklyTreemapCategory
}

// Rendered pixels per design unit, tracked from the container's width.
const scale = ref(1)
let observer: ResizeObserver | null = null
function observe(element: HTMLElement | null): void {
  observer?.disconnect()
  if (element === null || typeof ResizeObserver === 'undefined') return
  observer = new ResizeObserver(([entry]) => {
    if (entry) scale.value = entry.contentRect.width / WIDTH || 1
  })
  observer.observe(element)
}
onBeforeUnmount(() => observer?.disconnect())

function tileDetail(rect: Rect): TileDetail {
  const width = rect.width * scale.value
  const height = rect.height * scale.value
  if (width >= 110 && height >= 92) return 'full'
  if (width >= 76 && height >= 50) return 'medium'
  return 'compact'
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
          detail: tileDetail(tile),
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
          <b v-if="!narrowShell(entry.rect)">{{ formatDuration(entry.category.minutes) }}</b>
        </header>
      </section>
      <div
        v-for="tile in tiles"
        :key="tile.id"
        class="tm__tile"
        :class="{ 'is-compact': tile.detail === 'compact', 'is-hovered': hoveredId === tile.id }"
        :style="{ ...percentStyle(tile.rect), '--tm-color': tile.color }"
        @pointermove="pointer.move($event, tile)"
        @pointerleave="pointer.leave"
      >
        <AppSiteIcon v-if="tile.detail === 'full' && tile.app.sites.length > 0" :sites="tile.app.sites" :size="20" :accent="tile.color" />
        <span class="tm__name">{{ labels.app(tile.app.key, tile.app.name) }}</span>
        <span v-if="tile.detail !== 'compact'" class="tm__time">{{ formatDuration(tile.app.minutes) }}</span>
        <span
          v-if="tile.detail === 'full' && changeLabel(tile.app.changeMinutes)"
          class="tm__change"
          :class="(tile.app.changeMinutes ?? 0) > 0 ? 'is-up' : 'is-down'"
        >{{ changeLabel(tile.app.changeMinutes) }}</span>
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

.tm__name {
  max-width: 100%;
  overflow: hidden;
  color: var(--dg-wk-text);
  font-family: var(--dg-font-ui);
  font-size: 15.5px;
  font-weight: 650;
  letter-spacing: -0.01em;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tm__tile.is-compact .tm__name {
  font-size: 12px;
  font-weight: 650;
}

.tm__time,
.tm__change {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tm__time {
  color: color-mix(in srgb, var(--dg-wk-text) 78%, transparent);
  font-size: 13px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.tm__change {
  font-size: 12.5px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.tm__change.is-up { color: #1f8a57; }
.tm__change.is-down { color: #d8432c; }

:root[data-dg-appearance='dark'] .tm__change.is-up { color: #5fd49a; }
:root[data-dg-appearance='dark'] .tm__change.is-down { color: #ff8a73; }
</style>
