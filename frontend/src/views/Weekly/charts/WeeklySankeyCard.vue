<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { useDurationFormat } from '@/lib/duration'
import {
  SANKEY_BAR_WIDTH,
  SANKEY_COLUMNS,
  SANKEY_HEIGHT,
  SANKEY_WIDTH,
  sankeyGradientStops,
  sankeyLayout,
  sankeyRibbonPath,
  type SankeyFlow,
} from '@/lib/sankeyLayout'
import type { WeeklySankeySnapshot } from '@/stores/weeklyCharts'

import WeeklyChartCard from './WeeklyChartCard.vue'
import WeeklyChartTooltip from './WeeklyChartTooltip.vue'
import { useChartPointer } from './useChartPointer'
import { useWeeklyChartLabels } from './useWeeklyChartLabels'

/*
 * "Weekly breakdown" (Dayflow WeeklySankeyCard): the week opens outwards from
 * a short source bar into taller category bars and the tallest app column,
 * each app in its own colour and fed by every category it was used under.
 * Geometry lives in lib/sankeyLayout. As in Dayflow, hovering a node or a
 * ribbon (which stands for its target) keeps its connections and recedes the
 * rest; clicking pins it, clicking empty space unpins.
 *
 * Gradient ids are positional and scoped to this instance: names may contain
 * spaces ("Focus Work"), which once broke url(#…) and painted ribbons black.
 */
const props = defineProps<{ snapshot: WeeklySankeySnapshot; weekLabel: string; days: string[] }>()

const gradientPrefix = `sk-${useId()}`
const { t } = useI18n()
const formatDuration = useDurationFormat()
const labels = useWeeklyChartLabels(() => props.days)

const SOURCE_ID = 'source'
const categoryId = (key: string): string => `c:${key}`
const appId = (key: string): string => `a:${key}`

const categoryNodes = computed(() => new Map(props.snapshot.categories.map((node) => [node.key, node])))
const appNodes = computed(() => new Map(props.snapshot.apps.map((node) => [node.key, node])))
const total = computed(() => Math.max(1, props.snapshot.total))

const layout = computed(() => sankeyLayout(props.snapshot.categories, props.snapshot.apps, props.snapshot.links))

// Flows with namespaced endpoints, in Dayflow's terms: source → category → app.
const flows = computed(() =>
  layout.value.flows.map((flow) => ({
    ...flow,
    fromId: flow.kind === 'source' ? SOURCE_ID : categoryId(flow.from),
    toId: flow.kind === 'source' ? categoryId(flow.to) : appId(flow.to),
    path: sankeyRibbonPath(flow.x0, flow.y0Top, flow.y0Bottom, flow.x1, flow.y1Top, flow.y1Bottom, flow.tension),
    stops: sankeyGradientStops(flow),
  })),
)

// Warm washes under each pair of columns.
const underlays = computed(() => {
  const { categories, apps, source } = layout.value
  if (categories.length === 0 || apps.length === 0) return null
  const categoryTop = Math.min(...categories.map((band) => band.y))
  const categoryBottom = Math.max(...categories.map((band) => band.y + band.height))
  const appTop = Math.min(...apps.map((band) => band.y))
  const appBottom = Math.max(...apps.map((band) => band.y + band.height))
  const categoryX = SANKEY_COLUMNS.categories.x
  return {
    left: sankeyRibbonPath(source.x + SANKEY_BAR_WIDTH, source.y, source.y + source.height, categoryX, categoryTop, categoryBottom, 0.15),
    right: sankeyRibbonPath(categoryX + SANKEY_BAR_WIDTH, categoryTop, categoryBottom, SANKEY_COLUMNS.apps.x, appTop, appBottom, 0.22),
  }
})

// ---- Interaction (Dayflow WeeklySankeyCard.activeNodeID) -------------------

interface Hover {
  node: string
  flow: SankeyFlow | null
}

const pointer = useChartPointer<Hover>()
const pinned = ref<string | null>(null)
const active = computed(() => pinned.value ?? pointer.hovered.value?.node ?? null)

function flowRelated(fromId: string, toId: string): boolean {
  const current = active.value
  if (current === null || current === SOURCE_ID) return true
  return fromId === current || toId === current
}

function nodeRelated(id: string): boolean {
  const current = active.value
  if (current === null || current === SOURCE_ID || id === current) return true
  return flows.value.some((flow) =>
    (flow.fromId === current && flow.toId === id)
    || (flow.toId === current && flow.fromId === id)
    || (flow.fromId === SOURCE_ID && flow.toId === current && id === SOURCE_ID))
}

function toggle(id: string): void {
  pinned.value = pinned.value === id ? null : id
}

function share(minutes: number, of = total.value): string {
  if (of <= 0) return '0%'
  return `${Math.max(1, Math.round((minutes / of) * 100))}%`
}

const tooltip = computed(() => {
  const current = pointer.hovered.value
  if (current === null) return null
  const flow = current.flow
  if (flow !== null && flow.kind === 'link') {
    const from = categoryNodes.value.get(flow.from)
    const to = appNodes.value.get(flow.to)
    if (from === undefined || to === undefined) return null
    return {
      title: `${labels.category(from.key)} → ${labels.app(to.key, to.name)}`,
      lines: [
        formatDuration(flow.minutes),
        t('weekly.charts.tooltip.share', { name: labels.category(from.key), value: share(flow.minutes, from.minutes) }),
      ],
    }
  }
  if (current.node === SOURCE_ID) return { title: props.weekLabel, lines: [formatDuration(props.snapshot.total)] }
  const key = current.node.slice(2)
  if (current.node.startsWith('c:')) {
    const node = categoryNodes.value.get(key)
    return node ? { title: labels.category(node.key), lines: [`${formatDuration(node.minutes)} · ${share(node.minutes)}`] } : null
  }
  const node = appNodes.value.get(key)
  return node ? { title: labels.app(node.key, node.name), lines: [`${formatDuration(node.minutes)} · ${share(node.minutes)}`] } : null
})

// Labels are centred on their slot (top + half the slot height), so their
// text never has to fit a fixed box when the chart is narrow.
function slot(x: number, labelY: number, labelHeight: number, width: number): Record<string, string> {
  return {
    left: `${(x / SANKEY_WIDTH) * 100}%`,
    top: `${((labelY + labelHeight / 2) / SANKEY_HEIGHT) * 100}%`,
    maxWidth: `${(width / SANKEY_WIDTH) * 100}%`,
  }
}
</script>

<template>
  <WeeklyChartCard :title="t('weekly.charts.sankey.title')">
    <div
      :ref="(el) => { pointer.container.value = el as HTMLElement | null }"
      class="sk"
      :class="{ 'has-focus': active !== null }"
      role="img"
      :aria-label="t('weekly.charts.sankey.aria')"
      :title="t('weekly.charts.tooltip.pinHint')"
      @click="pinned = null"
    >
      <svg :viewBox="`0 0 ${SANKEY_WIDTH} ${SANKEY_HEIGHT}`" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient :id="`${gradientPrefix}-ul`" x1="0" x2="1" y1="0" y2="0">
            <stop offset="0" stop-color="#E6DBD1" stop-opacity="0.48" />
            <stop offset="0.42" stop-color="#EFE9E3" stop-opacity="0.34" />
            <stop offset="0.76" stop-color="#F4EEE9" stop-opacity="0.2" />
            <stop offset="1" stop-color="#F7F2ED" stop-opacity="0.08" />
          </linearGradient>
          <linearGradient :id="`${gradientPrefix}-ur`" x1="0" x2="1" y1="0" y2="0">
            <stop offset="0" stop-color="#EFE7E0" stop-opacity="0.08" />
            <stop offset="0.46" stop-color="#F4EEE9" stop-opacity="0.11" />
            <stop offset="1" stop-color="#EFE7E0" stop-opacity="0.07" />
          </linearGradient>
          <linearGradient
            v-for="flow in flows"
            :id="`${gradientPrefix}-${flow.id}`"
            :key="`grad-${flow.id}`"
            gradientUnits="userSpaceOnUse"
            :x1="flow.x0"
            :x2="flow.x1"
            y1="0"
            y2="0"
          >
            <stop v-for="stop in flow.stops" :key="stop.offset" :offset="stop.offset" :stop-color="stop.color" :stop-opacity="stop.opacity" />
          </linearGradient>
        </defs>

        <g v-if="underlays" class="sk__underlay">
          <path :d="underlays.left" :fill="`url(#${gradientPrefix}-ul)`" />
          <path :d="underlays.right" :fill="`url(#${gradientPrefix}-ur)`" opacity="0.72" />
        </g>

        <path
          v-for="flow in flows"
          :key="flow.id"
          class="sk__ribbon"
          :class="{ 'is-dim': !flowRelated(flow.fromId, flow.toId) }"
          :d="flow.path"
          :fill="`url(#${gradientPrefix}-${flow.id})`"
          @pointermove="pointer.move($event, { node: flow.toId, flow })"
          @pointerleave="pointer.leave"
          @click.stop="toggle(flow.toId)"
        />

        <rect
          class="sk__bar"
          :class="{ 'is-dim': !nodeRelated(SOURCE_ID) }"
          :x="layout.source.x"
          :y="layout.source.y"
          :width="SANKEY_BAR_WIDTH"
          :height="layout.source.height"
          :fill="layout.source.colorHex"
          @pointermove="pointer.move($event, { node: SOURCE_ID, flow: null })"
          @pointerleave="pointer.leave"
          @click.stop="toggle(SOURCE_ID)"
        />
        <rect
          v-for="band in layout.categories"
          :key="`cat-${band.key}`"
          class="sk__bar"
          :class="{ 'is-dim': !nodeRelated(categoryId(band.key)) }"
          :x="band.x"
          :y="band.y"
          :width="SANKEY_BAR_WIDTH"
          :height="band.height"
          :fill="band.colorHex"
          @pointermove="pointer.move($event, { node: categoryId(band.key), flow: null })"
          @pointerleave="pointer.leave"
          @click.stop="toggle(categoryId(band.key))"
        />
        <rect
          v-for="band in layout.apps"
          :key="`app-${band.key}`"
          class="sk__bar"
          :class="{ 'is-dim': !nodeRelated(appId(band.key)) }"
          :x="band.x"
          :y="band.y"
          :width="SANKEY_BAR_WIDTH"
          :height="band.height"
          :fill="band.colorHex"
          @pointermove="pointer.move($event, { node: appId(band.key), flow: null })"
          @pointerleave="pointer.leave"
          @click.stop="toggle(appId(band.key))"
        />
      </svg>

      <div
        class="sk__label"
        :class="{ 'is-dim': !nodeRelated(SOURCE_ID) }"
        :style="slot(SANKEY_COLUMNS.source.labelX, layout.source.labelY, SANKEY_COLUMNS.source.labelHeight, 220)"
        @pointermove="pointer.move($event, { node: SOURCE_ID, flow: null })"
        @pointerleave="pointer.leave"
        @click.stop="toggle(SOURCE_ID)"
      >
        <b>{{ weekLabel }}</b>
        <span class="sk__meta">{{ formatDuration(snapshot.total) }}<i></i>100%</span>
      </div>
      <div
        v-for="band in layout.categories"
        :key="`label-cat-${band.key}`"
        class="sk__label"
        :class="{ 'is-dim': !nodeRelated(categoryId(band.key)) }"
        :style="slot(SANKEY_COLUMNS.categories.labelX, band.labelY, SANKEY_COLUMNS.categories.labelHeight, 260)"
        @pointermove="pointer.move($event, { node: categoryId(band.key), flow: null })"
        @pointerleave="pointer.leave"
        @click.stop="toggle(categoryId(band.key))"
      >
        <b>{{ labels.category(band.key) }}</b>
        <span class="sk__meta">{{ formatDuration(band.minutes) }}<i></i>{{ share(band.minutes) }}</span>
      </div>
      <div
        v-for="band in layout.apps"
        :key="`label-app-${band.key}`"
        class="sk__label sk__label--app"
        :class="{ 'is-dim': !nodeRelated(appId(band.key)) }"
        :style="slot(SANKEY_COLUMNS.apps.labelX, band.labelY, SANKEY_COLUMNS.apps.labelHeight, SANKEY_WIDTH - SANKEY_COLUMNS.apps.labelX)"
        @pointermove="pointer.move($event, { node: appId(band.key), flow: null })"
        @pointerleave="pointer.leave"
        @click.stop="toggle(appId(band.key))"
      >
        <AppSiteIcon
          v-if="(appNodes.get(band.key)?.sites.length ?? 0) > 0"
          :sites="appNodes.get(band.key)!.sites"
          :size="15"
          :accent="band.colorHex"
        />
        <b>{{ labels.app(band.key, appNodes.get(band.key)?.name ?? '') }}</b>
        <span class="sk__meta">{{ formatDuration(band.minutes) }}<i></i>{{ share(band.minutes) }}</span>
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
  container-type: inline-size;
  width: 100%;
  aspect-ratio: 1748 / 933;
  min-height: 360px;
}

.sk svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.sk__underlay {
  opacity: var(--dg-wk-sankey-underlay);
  pointer-events: none;
}

.sk__ribbon,
.sk__bar,
.sk__label {
  transition: opacity 260ms cubic-bezier(0.22, 1, 0.36, 1);
}

.sk__ribbon {
  cursor: pointer;
}

.sk__ribbon.is-dim {
  opacity: 0.12;
}

.sk__bar {
  cursor: pointer;
}

.sk__bar.is-dim,
.sk__label.is-dim {
  opacity: 0.25;
}

.sk__label {
  position: absolute;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  cursor: pointer;
  transform: translateY(-50%);
}

.sk__label b {
  overflow: hidden;
  color: var(--dg-wk-text);
  /* Scales with the chart: 13.5px on a full-width card, never below 11px. */
  font-size: clamp(11px, 1.2cqw, 13.5px);
  font-weight: 600;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sk__meta {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--dg-wk-text-secondary);
  font-size: clamp(10px, 1.06cqw, 12px);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* Dayflow's hairline between the duration and the share. */
.sk__meta i {
  width: 1px;
  height: 11px;
  background: var(--dg-wk-divider);
}

.sk__label--app {
  flex-direction: row;
  align-items: center;
  justify-content: flex-start;
  gap: 6px;
}

.sk__label--app b {
  flex: 0 1 auto;
  min-width: 2.5em;
}

.sk__label--app .sk__meta {
  flex: none;
  font-size: clamp(9.5px, 1.02cqw, 11.5px);
}

@media (prefers-reduced-motion: reduce) {
  .sk__ribbon,
  .sk__bar,
  .sk__label {
    transition: none;
  }
}
</style>
