<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { squarify, type Rect } from '@/lib/chartLayout'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklyTreemapApp, WeeklyTreemapCategory } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
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

interface TilePlacement {
  app: WeeklyTreemapApp
  rect: Rect
  roomy: boolean
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
        .map(({ item: app, rect: tile }) => ({ app, rect: tile, roomy: tile.width >= 70 && tile.height >= 44 }))
      return { category, rect, tiles }
    }),
)

function changeLabel(minutes: number | null): string | null {
  if (minutes === null || minutes === 0) return null
  return `${minutes > 0 ? '+' : '−'} ${formatDuration(Math.abs(minutes))}`
}
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.treemap.title')">
    <p v-if="categories.length === 0" class="tm__empty">{{ t('weekly.charts.treemap.empty') }}</p>
    <div v-else class="tm" role="img" :aria-label="t('weekly.charts.treemap.aria')">
      <section
        v-for="entry in layout"
        :key="entry.category.name"
        class="tm__shell"
        :style="{ ...percentStyle(entry.rect), '--tm-color': entry.category.colorHex }"
      >
        <header class="tm__header">
          <span>{{ labels.category(entry.category.name) }}</span>
          <b>{{ formatDuration(entry.category.minutes) }}</b>
        </header>
      </section>
      <div
        v-for="tile in layout.flatMap((entry) => entry.tiles.map((placed) => ({ ...placed, color: entry.category.colorHex, category: entry.category.name })))"
        :key="`${tile.category}-${tile.app.key}`"
        class="tm__tile"
        :class="{ 'is-compact': !tile.roomy }"
        :style="{ ...percentStyle(tile.rect), '--tm-color': tile.color }"
        :title="`${labels.app(tile.app.key, tile.app.name)} · ${formatDuration(tile.app.minutes)}`"
      >
        <AppSiteIcon v-if="tile.roomy && tile.app.sites.length > 0" :sites="tile.app.sites" :size="16" :accent="tile.color" />
        <span class="tm__name">{{ labels.app(tile.app.key, tile.app.name) }}</span>
        <span v-if="tile.roomy" class="tm__time">{{ formatDuration(tile.app.minutes) }}</span>
        <span
          v-if="tile.roomy && changeLabel(tile.app.changeMinutes)"
          class="tm__change"
          :class="(tile.app.changeMinutes ?? 0) > 0 ? 'is-up' : 'is-down'"
        >{{ changeLabel(tile.app.changeMinutes) }}</span>
      </div>
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
  color: #6d6d6d;
  font-size: 11px;
  white-space: nowrap;
}

:root[data-dg-appearance='dark'] .tm__header { color: #dfdfdf; }

.tm__header span {
  overflow: hidden;
  text-overflow: ellipsis;
}

.tm__header b {
  flex: none;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.tm__tile {
  position: absolute;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3px;
  padding: 4px;
  overflow: hidden;
  border: 1px solid var(--tm-color);
  border-radius: 4px;
  background: color-mix(in srgb, var(--tm-color) 42%, transparent);
  color: var(--dg-wk-text);
  text-align: center;
}

.tm__name {
  max-width: 100%;
  overflow: hidden;
  font-family: var(--dg-font-serif);
  font-size: 15px;
  line-height: 1.1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tm__tile.is-compact .tm__name {
  font-family: var(--dg-font-ui);
  font-size: 10px;
}

.tm__time {
  color: var(--dg-wk-text-secondary);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}

.tm__change {
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}

.tm__change.is-up { color: #2e9e6a; }
.tm__change.is-down { color: #e5533d; }
</style>
