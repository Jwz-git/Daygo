<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { ribbonPath, stackColumn } from '@/lib/chartLayout'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklySankeySnapshot } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'

/*
 * "Weekly breakdown" (Dayflow WeeklySankeySection): the week's time flows
 * from one source bar into category bars, then into app bars. Ribbons fade
 * from a warm neutral into the target colour, stronger for bigger flows.
 */
const props = defineProps<{ snapshot: WeeklySankeySnapshot; weekLabel: string; days: string[] }>()

const { t } = useI18n()
const formatDuration = useDurationFormat()
const labels = useWeeklyChartLabels(() => props.days)

const WIDTH = 900
const HEIGHT = 380
const TOP = 10
const BOTTOM = HEIGHT - 10
const BAR = 8
const SOURCE_X = 4
const CATEGORY_X = 320
const APP_X = 640

const total = computed(() => Math.max(1, props.snapshot.total))
const categoryNodes = computed(() => new Map(props.snapshot.categories.map((node) => [node.key, node])))
const appNodes = computed(() => new Map(props.snapshot.apps.map((node) => [node.key, node])))
// An app bar thinner than two text lines gets a single-line label.
const COMPACT_LABEL_HEIGHT = 26

// App nodes take the colour of the category that feeds them most.
const appColors = computed(() => {
  const best = new Map<string, { minutes: number; color: string }>()
  const categoryColor = new Map(props.snapshot.categories.map((node) => [node.key, node.colorHex]))
  for (const link of props.snapshot.links) {
    const current = best.get(link.to)
    if (current === undefined || link.minutes > current.minutes) {
      best.set(link.to, { minutes: link.minutes, color: categoryColor.get(link.from) ?? '#BFB6AE' })
    }
  }
  return new Map([...best.entries()].map(([key, value]) => [key, value.color]))
})

const layout = computed(() => {
  const categories = stackColumn(props.snapshot.categories, TOP, BOTTOM, 8)
  const apps = stackColumn(props.snapshot.apps, TOP, BOTTOM, 6)
  const source = { y: TOP, height: BOTTOM - TOP }
  const categoryByKey = new Map(categories.map((node) => [node.key, node]))
  const appByKey = new Map(apps.map((node) => [node.key, node]))

  // Source → category ribbons, stacked on the source bar in category order.
  const unitSource = source.height / total.value
  let sourceCursor = source.y
  const sourceRibbons = props.snapshot.categories.map((node) => {
    const target = categoryByKey.get(node.key)!
    const height = node.minutes * unitSource
    const path = ribbonPath(SOURCE_X + BAR, sourceCursor, height, CATEGORY_X, target.y, target.height)
    sourceCursor += height
    return { key: `source-${node.key}`, path, color: node.colorHex, strength: Math.sqrt(node.minutes / total.value) }
  })

  // Category → app ribbons: each link takes the next slice of both bars.
  const categoryCursor = new Map(categories.map((node) => [node.key, node.y]))
  const appCursor = new Map(apps.map((node) => [node.key, node.y]))
  const maxLink = Math.max(1, ...props.snapshot.links.map((link) => link.minutes))
  const links = [...props.snapshot.links]
    .sort((left, right) => right.minutes - left.minutes)
    .map((link) => {
      const from = categoryByKey.get(link.from)
      const to = appByKey.get(link.to)
      if (from === undefined || to === undefined) return null
      const fromNode = categoryNodes.value.get(link.from)!
      const toNode = appNodes.value.get(link.to)!
      const fromHeight = (link.minutes / Math.max(1, fromNode.minutes)) * from.height
      const toHeight = (link.minutes / Math.max(1, toNode.minutes)) * to.height
      const y0 = categoryCursor.get(link.from)!
      const y1 = appCursor.get(link.to)!
      categoryCursor.set(link.from, y0 + fromHeight)
      appCursor.set(link.to, y1 + toHeight)
      return {
        key: `${link.from}-${link.to}`,
        path: ribbonPath(CATEGORY_X + BAR, y0, fromHeight, APP_X, y1, toHeight),
        color: appColors.value.get(link.to) ?? fromNode.colorHex,
        strength: Math.sqrt(link.minutes / maxLink),
      }
    })
    .filter((ribbon): ribbon is NonNullable<typeof ribbon> => ribbon !== null)

  return { source, categories, apps, sourceRibbons, links }
})

function nodeTop(y: number, height: number): string {
  return `${((y + height / 2) / HEIGHT) * 100}%`
}

function share(minutes: number): string {
  return `${Math.round((minutes / total.value) * 100)}%`
}
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.sankey.title')">
    <div class="sk" role="img" :aria-label="t('weekly.charts.sankey.aria')">
      <svg :viewBox="`0 0 ${WIDTH} ${HEIGHT}`" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient
            v-for="ribbon in [...layout.sourceRibbons, ...layout.links]"
            :id="`sk-${ribbon.key}`"
            :key="`grad-${ribbon.key}`"
            x1="0"
            x2="1"
            y1="0"
            y2="0"
          >
            <stop offset="0" stop-color="#E3D8CF" :stop-opacity="0.18" />
            <stop offset="0.58" :stop-color="ribbon.color" :stop-opacity="Math.min(0.12, ribbon.strength * 0.42)" />
            <stop offset="1" :stop-color="ribbon.color" :stop-opacity="Math.max(0.12, Math.min(0.36, ribbon.strength * 1.08))" />
          </linearGradient>
        </defs>
        <path v-for="ribbon in layout.sourceRibbons" :key="ribbon.key" :d="ribbon.path" :fill="`url(#sk-${ribbon.key})`" />
        <path v-for="ribbon in layout.links" :key="ribbon.key" :d="ribbon.path" :fill="`url(#sk-${ribbon.key})`" />
        <rect class="sk__source" :x="SOURCE_X" :y="layout.source.y" :width="BAR" :height="layout.source.height" rx="2" />
        <rect
          v-for="node in layout.categories"
          :key="`cat-${node.key}`"
          :x="CATEGORY_X"
          :y="node.y"
          :width="BAR"
          :height="node.height"
          rx="2"
          :fill="categoryNodes.get(node.key)?.colorHex"
        />
        <rect
          v-for="node in layout.apps"
          :key="`app-${node.key}`"
          :x="APP_X"
          :y="node.y"
          :width="BAR"
          :height="node.height"
          rx="2"
          :fill="appColors.get(node.key) ?? '#BFB6AE'"
        />
      </svg>
      <div class="sk__label sk__label--source" :style="{ top: '50%', left: `${((SOURCE_X + BAR + 8) / WIDTH) * 100}%` }">
        <b>{{ weekLabel }}</b>
        <span>{{ formatDuration(snapshot.total) }}</span>
      </div>
      <div
        v-for="node in layout.categories"
        :key="`label-cat-${node.key}`"
        class="sk__label"
        :style="{ top: nodeTop(node.y, node.height), left: `${((CATEGORY_X + BAR + 8) / WIDTH) * 100}%` }"
      >
        <b>{{ labels.category(node.key) }}</b>
        <span>
          {{ formatDuration(categoryNodes.get(node.key)?.minutes ?? 0) }}
          · {{ share(categoryNodes.get(node.key)?.minutes ?? 0) }}
        </span>
      </div>
      <div
        v-for="node in layout.apps"
        :key="`label-app-${node.key}`"
        class="sk__label sk__label--app"
        :class="{ 'is-compact': node.height < COMPACT_LABEL_HEIGHT }"
        :style="{ top: nodeTop(node.y, node.height), left: `${((APP_X + BAR + 8) / WIDTH) * 100}%` }"
      >
        <AppSiteIcon
          v-if="(appNodes.get(node.key)?.sites.length ?? 0) > 0"
          :sites="appNodes.get(node.key)!.sites"
          :size="14"
          :accent="appColors.get(node.key)"
        />
        <span class="sk__app-text">
          <b>{{ labels.app(node.key, appNodes.get(node.key)?.name ?? '') }}</b>
          <span>
            {{ formatDuration(appNodes.get(node.key)?.minutes ?? 0) }}
            · {{ share(appNodes.get(node.key)?.minutes ?? 0) }}
          </span>
        </span>
      </div>
    </div>
  </WeeklyChartCard>
</template>

<style scoped>
.sk {
  position: relative;
  width: 100%;
  aspect-ratio: 900 / 380;
  min-height: 300px;
}

.sk svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.sk__source {
  fill: #d6c6b8;
}

.sk__label {
  position: absolute;
  display: flex;
  flex-direction: column;
  max-width: 22%;
  transform: translateY(-50%);
  pointer-events: none;
}

.sk__label b {
  overflow: hidden;
  color: var(--dg-wk-text);
  font-size: 11px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sk__label span {
  color: var(--dg-wk-text-muted);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.sk__label--app {
  flex-direction: row;
  align-items: center;
  gap: 6px;
  max-width: 28%;
}

.sk__app-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

/* Thin bars: name and duration on one line so neighbours never overlap. */
.sk__label--app.is-compact .sk__app-text {
  flex-direction: row;
  align-items: baseline;
  gap: 6px;
}
</style>
