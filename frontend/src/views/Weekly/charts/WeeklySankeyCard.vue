<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { ribbonPath, stackColumn } from '@/lib/chartLayout'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklySankeySnapshot } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
import WeeklyChartTooltip from './WeeklyChartTooltip.vue'
import { useChartPointer } from './useChartPointer'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'

/*
 * "Weekly breakdown" (Dayflow WeeklySankeySection): the week flows from one
 * source bar into category bars, then into app bars. Ribbons fade from a warm
 * neutral into the target colour. As in Dayflow, hovering a node or a ribbon
 * lights its whole path and lets the rest recede; clicking pins it.
 *
 * Ribbons within a bar are stacked in the order of the bars they connect to,
 * which keeps crossings to the ones the data actually forces. Gradient ids are
 * positional: category names may contain spaces ("Focus Work"), which broke
 * url(#…) references and painted the ribbons black.
 */
const props = defineProps<{ snapshot: WeeklySankeySnapshot; weekLabel: string; days: string[] }>()

// Gradient ids are positional and scoped to this instance, never built from names.
const gradientPrefix = `sk-${useId()}`

const { t } = useI18n()
const formatDuration = useDurationFormat()
const labels = useWeeklyChartLabels(() => props.days)

const WIDTH = 900
const HEIGHT = 400
const TOP = 10
const BOTTOM = HEIGHT - 10
const BAR = 9
const SOURCE_X = 4
const CATEGORY_X = 330
const APP_X = 650
const COMPACT_LABEL_HEIGHT = 28

const total = computed(() => Math.max(1, props.snapshot.total))
const categoryNodes = computed(() => new Map(props.snapshot.categories.map((node) => [node.key, node])))
const appNodes = computed(() => new Map(props.snapshot.apps.map((node) => [node.key, node])))

// App bars take the colour of the category that feeds them most.
const appColors = computed(() => {
  const best = new Map<string, { minutes: number; color: string }>()
  for (const link of props.snapshot.links) {
    const current = best.get(link.to)
    if (current === undefined || link.minutes > current.minutes) {
      best.set(link.to, { minutes: link.minutes, color: categoryNodes.value.get(link.from)?.colorHex ?? '#BFB6AE' })
    }
  }
  return new Map([...best.entries()].map(([key, value]) => [key, value.color]))
})

interface Ribbon {
  id: string
  kind: 'source' | 'link'
  from: string
  to: string
  minutes: number
  path: string
  color: string
  strength: number
}

const layout = computed(() => {
  const categories = stackColumn(props.snapshot.categories, TOP, BOTTOM, 10)
  const apps = stackColumn(props.snapshot.apps, TOP, BOTTOM, 7)
  const categoryByKey = new Map(categories.map((node) => [node.key, node]))
  const appByKey = new Map(apps.map((node) => [node.key, node]))
  const categoryOrder = new Map(categories.map((node, index) => [node.key, index]))
  const appOrder = new Map(apps.map((node, index) => [node.key, index]))
  const ribbons: Ribbon[] = []

  let sourceCursor = TOP
  const unit = (BOTTOM - TOP) / total.value
  props.snapshot.categories.forEach((node) => {
    const target = categoryByKey.get(node.key)
    if (target === undefined) return
    const height = node.minutes * unit
    ribbons.push({
      id: `r${ribbons.length}`,
      kind: 'source',
      from: '',
      to: node.key,
      minutes: node.minutes,
      path: ribbonPath(SOURCE_X + BAR, sourceCursor, height, CATEGORY_X, target.y, target.height),
      color: node.colorHex,
      strength: Math.sqrt(node.minutes / total.value),
    })
    sourceCursor += height
  })

  // Out of a category: ordered by the target app's position. Into an app:
  // ordered by the source category's position.
  const links = props.snapshot.links.filter((link) => categoryByKey.has(link.from) && appByKey.has(link.to))
  const outCursor = new Map<string, number>()
  const startOf = new Map<string, number>()
  const outgoing = [...links].sort((a, b) =>
    (categoryOrder.get(a.from)! - categoryOrder.get(b.from)!) || (appOrder.get(a.to)! - appOrder.get(b.to)!))
  for (const link of outgoing) {
    const node = categoryByKey.get(link.from)!
    const offset = outCursor.get(link.from) ?? node.y
    outCursor.set(link.from, offset + (link.minutes / Math.max(1, categoryNodes.value.get(link.from)!.minutes)) * node.height)
    startOf.set(`${link.from}\u0000${link.to}`, offset)
  }
  const incoming = [...links].sort((a, b) =>
    (appOrder.get(a.to)! - appOrder.get(b.to)!) || (categoryOrder.get(a.from)! - categoryOrder.get(b.from)!))
  const inCursor = new Map<string, number>()
  const maxLink = Math.max(1, ...links.map((link) => link.minutes))
  for (const link of incoming) {
    const from = categoryByKey.get(link.from)!
    const to = appByKey.get(link.to)!
    const fromHeight = (link.minutes / Math.max(1, categoryNodes.value.get(link.from)!.minutes)) * from.height
    const toHeight = (link.minutes / Math.max(1, appNodes.value.get(link.to)!.minutes)) * to.height
    const y0 = startOf.get(`${link.from}\u0000${link.to}`)!
    const y1 = inCursor.get(link.to) ?? to.y
    inCursor.set(link.to, y1 + toHeight)
    ribbons.push({
      id: `r${ribbons.length}`,
      kind: 'link',
      from: link.from,
      to: link.to,
      minutes: link.minutes,
      path: ribbonPath(CATEGORY_X + BAR, y0, fromHeight, APP_X, y1, toHeight),
      color: appColors.value.get(link.to) ?? categoryNodes.value.get(link.from)!.colorHex,
      strength: Math.sqrt(link.minutes / maxLink),
    })
  }
  return { categories, apps, ribbons }
})

// ---- Interaction ---------------------------------------------------------

type Focus =
  | { type: 'source' }
  | { type: 'category'; key: string }
  | { type: 'app'; key: string }
  | { type: 'link'; from: string; to: string }

const pointer = useChartPointer<Focus>()
const pinned = ref<Focus | null>(null)
const focus = computed<Focus | null>(() => pinned.value ?? pointer.hovered.value)

function sameFocus(a: Focus | null, b: Focus | null): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}

function ribbonFocus(ribbon: Ribbon): Focus {
  return ribbon.kind === 'source' ? { type: 'category', key: ribbon.to } : { type: 'link', from: ribbon.from, to: ribbon.to }
}

// Which ribbons, categories and apps belong to the focused path.
const lit = computed(() => {
  const current = focus.value
  if (current === null) return null
  const ribbons = new Set<string>()
  const categories = new Set<string>()
  const apps = new Set<string>()
  for (const ribbon of layout.value.ribbons) {
    let on = false
    if (current.type === 'source') on = true
    else if (current.type === 'category') on = ribbon.kind === 'source' ? ribbon.to === current.key : ribbon.from === current.key
    else if (current.type === 'app') {
      on = ribbon.kind === 'link'
        ? ribbon.to === current.key
        : props.snapshot.links.some((link) => link.from === ribbon.to && link.to === current.key)
    } else {
      on = ribbon.kind === 'link' ? ribbon.from === current.from && ribbon.to === current.to : ribbon.to === current.from
    }
    if (!on) continue
    ribbons.add(ribbon.id)
    if (ribbon.kind === 'source') categories.add(ribbon.to)
    else {
      categories.add(ribbon.from)
      apps.add(ribbon.to)
    }
  }
  return { ribbons, categories, apps }
})

function stateOf(set: 'ribbons' | 'categories' | 'apps', key: string): string {
  if (lit.value === null) return ''
  return lit.value[set].has(key) ? 'is-lit' : 'is-dim'
}

function toggle(target: Focus): void {
  pinned.value = sameFocus(pinned.value, target) ? null : target
}

function share(minutes: number, of = total.value): string {
  return `${Math.round((minutes / Math.max(1, of)) * 100)}%`
}

const tooltip = computed(() => {
  const current = pointer.hovered.value
  if (current === null) return null
  if (current.type === 'source') return { title: props.weekLabel, lines: [formatDuration(props.snapshot.total)] }
  if (current.type === 'category') {
    const node = categoryNodes.value.get(current.key)
    if (node === undefined) return null
    return { title: labels.category(node.key), lines: [`${formatDuration(node.minutes)} · ${share(node.minutes)}`] }
  }
  if (current.type === 'app') {
    const node = appNodes.value.get(current.key)
    if (node === undefined) return null
    return { title: labels.app(node.key, node.name), lines: [`${formatDuration(node.minutes)} · ${share(node.minutes)}`] }
  }
  const link = props.snapshot.links.find((entry) => entry.from === current.from && entry.to === current.to)
  const from = categoryNodes.value.get(current.from)
  const to = appNodes.value.get(current.to)
  if (link === undefined || from === undefined || to === undefined) return null
  return {
    title: `${labels.category(from.key)} → ${labels.app(to.key, to.name)}`,
    lines: [
      formatDuration(link.minutes),
      t('weekly.charts.tooltip.share', { name: labels.category(from.key), value: share(link.minutes, from.minutes) }),
    ],
  }
})

function nodeTop(y: number, height: number): string {
  return `${((y + height / 2) / HEIGHT) * 100}%`
}
function columnLeft(x: number): string {
  return `${((x + BAR + 10) / WIDTH) * 100}%`
}
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.sankey.title')">
    <div
      :ref="(el) => { pointer.container.value = el as HTMLElement | null }"
      class="sk"
      role="img"
      :aria-label="t('weekly.charts.sankey.aria')"
      :title="t('weekly.charts.tooltip.pinHint')"
      @click="pinned = null"
    >
      <svg :viewBox="`0 0 ${WIDTH} ${HEIGHT}`" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <template v-for="ribbon in layout.ribbons" :key="`grad-${ribbon.id}`">
            <linearGradient :id="`${gradientPrefix}-${ribbon.id}`" x1="0" x2="1" y1="0" y2="0">
              <stop offset="0" stop-color="#E3D8CF" stop-opacity="0.2" />
              <stop offset="0.55" :stop-color="ribbon.color" :stop-opacity="Math.min(0.14, ribbon.strength * 0.45)" />
              <stop offset="1" :stop-color="ribbon.color" :stop-opacity="Math.max(0.14, Math.min(0.38, ribbon.strength * 1.1))" />
            </linearGradient>
            <linearGradient :id="`${gradientPrefix}-${ribbon.id}-lit`" x1="0" x2="1" y1="0" y2="0">
              <stop offset="0" :stop-color="ribbon.color" stop-opacity="0.22" />
              <stop offset="1" :stop-color="ribbon.color" stop-opacity="0.62" />
            </linearGradient>
          </template>
        </defs>
        <g
          v-for="ribbon in layout.ribbons"
          :key="ribbon.id"
          class="sk__ribbon"
          :class="stateOf('ribbons', ribbon.id)"
          @pointermove="pointer.move($event, ribbonFocus(ribbon))"
          @pointerleave="pointer.leave"
          @click.stop="toggle(ribbonFocus(ribbon))"
        >
          <path :d="ribbon.path" :fill="`url(#${gradientPrefix}-${ribbon.id})`" />
          <path class="sk__ribbon-lit" :d="ribbon.path" :fill="`url(#${gradientPrefix}-${ribbon.id}-lit)`" />
        </g>
        <rect
          class="sk__bar sk__source"
          :x="SOURCE_X"
          :y="TOP"
          :width="BAR"
          :height="BOTTOM - TOP"
          rx="2"
          @pointermove="pointer.move($event, { type: 'source' })"
          @pointerleave="pointer.leave"
          @click.stop="toggle({ type: 'source' })"
        />
        <rect
          v-for="node in layout.categories"
          :key="`cat-${node.key}`"
          class="sk__bar"
          :class="stateOf('categories', node.key)"
          :x="CATEGORY_X"
          :y="node.y"
          :width="BAR"
          :height="node.height"
          rx="2"
          :fill="categoryNodes.get(node.key)?.colorHex"
          @pointermove="pointer.move($event, { type: 'category', key: node.key })"
          @pointerleave="pointer.leave"
          @click.stop="toggle({ type: 'category', key: node.key })"
        />
        <rect
          v-for="node in layout.apps"
          :key="`app-${node.key}`"
          class="sk__bar"
          :class="stateOf('apps', node.key)"
          :x="APP_X"
          :y="node.y"
          :width="BAR"
          :height="node.height"
          rx="2"
          :fill="appColors.get(node.key) ?? '#BFB6AE'"
          @pointermove="pointer.move($event, { type: 'app', key: node.key })"
          @pointerleave="pointer.leave"
          @click.stop="toggle({ type: 'app', key: node.key })"
        />
      </svg>

      <div class="sk__label sk__label--source" :style="{ top: '50%', left: columnLeft(SOURCE_X) }">
        <b>{{ weekLabel }}</b>
        <span>{{ formatDuration(snapshot.total) }}</span>
      </div>
      <div
        v-for="node in layout.categories"
        :key="`label-cat-${node.key}`"
        class="sk__label sk__label--node"
        :class="stateOf('categories', node.key)"
        :style="{ top: nodeTop(node.y, node.height), left: columnLeft(CATEGORY_X) }"
        @pointermove="pointer.move($event, { type: 'category', key: node.key })"
        @pointerleave="pointer.leave"
        @click.stop="toggle({ type: 'category', key: node.key })"
      >
        <b>{{ labels.category(node.key) }}</b>
        <span>{{ formatDuration(categoryNodes.get(node.key)?.minutes ?? 0) }} · {{ share(categoryNodes.get(node.key)?.minutes ?? 0) }}</span>
      </div>
      <div
        v-for="node in layout.apps"
        :key="`label-app-${node.key}`"
        class="sk__label sk__label--node sk__label--app"
        :class="[stateOf('apps', node.key), { 'is-compact': node.height < COMPACT_LABEL_HEIGHT }]"
        :style="{ top: nodeTop(node.y, node.height), left: columnLeft(APP_X) }"
        @pointermove="pointer.move($event, { type: 'app', key: node.key })"
        @pointerleave="pointer.leave"
        @click.stop="toggle({ type: 'app', key: node.key })"
      >
        <AppSiteIcon
          v-if="(appNodes.get(node.key)?.sites.length ?? 0) > 0"
          :sites="appNodes.get(node.key)!.sites"
          :size="18"
          :accent="appColors.get(node.key)"
        />
        <span class="sk__app-text">
          <b>{{ labels.app(node.key, appNodes.get(node.key)?.name ?? '') }}</b>
          <span>{{ formatDuration(appNodes.get(node.key)?.minutes ?? 0) }} · {{ share(appNodes.get(node.key)?.minutes ?? 0) }}</span>
        </span>
      </div>

      <WeeklyChartTooltip :visible="tooltip !== null" :x="pointer.x.value" :y="pointer.y.value">
        <template v-if="tooltip">
          <b>{{ tooltip.title }}</b>
          <span v-for="line in tooltip.lines" :key="line">{{ line }}</span>
        </template>
      </WeeklyChartTooltip>
    </div>
  </WeeklyChartCard>
</template>

<style scoped>
.sk {
  position: relative;
  width: 100%;
  aspect-ratio: 900 / 400;
  min-height: 320px;
}

.sk svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.sk__ribbon {
  cursor: pointer;
  transition: opacity 240ms cubic-bezier(0.22, 1, 0.36, 1);
}

.sk__ribbon-lit {
  opacity: 0;
  transition: opacity 240ms cubic-bezier(0.22, 1, 0.36, 1);
}

.sk__ribbon.is-lit .sk__ribbon-lit { opacity: 1; }
.sk__ribbon.is-dim { opacity: 0.12; }

.sk__bar {
  cursor: pointer;
  transition: opacity 240ms cubic-bezier(0.22, 1, 0.36, 1);
}

.sk__bar.is-dim { opacity: 0.3; }

.sk__source {
  fill: #d6c6b8;
}

.sk__label {
  position: absolute;
  display: flex;
  flex-direction: column;
  max-width: 22%;
  transform: translateY(-50%);
  transition: opacity 240ms cubic-bezier(0.22, 1, 0.36, 1);
}

.sk__label--node {
  cursor: pointer;
}

.sk__label.is-dim { opacity: 0.35; }

.sk__label b {
  overflow: hidden;
  color: var(--dg-wk-text);
  font-size: 13.5px;
  font-weight: 650;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sk__label span {
  color: var(--dg-wk-text-secondary);
  font-size: 12px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.sk__label--app {
  flex-direction: row;
  align-items: center;
  gap: 8px;
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

@media (prefers-reduced-motion: reduce) {
  .sk__ribbon,
  .sk__ribbon-lit,
  .sk__bar,
  .sk__label {
    transition: none;
  }
}
</style>
