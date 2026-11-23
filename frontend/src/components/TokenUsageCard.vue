<script setup lang="ts">
import { computed, ref, toRef, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DgSelect, { type DgSelectOption } from '@/components/DgSelect.vue'
import { useTokenUsage } from '@/stores/tokenUsage'
import { tokenUsageFormatter, tokenUsageHoverIndex, tokenUsagePoints, tokenUsageTotals } from '@/lib/tokenUsageChart'

import { safeTimeZone } from '@/lib/timeZone'
import { svgPoint, useChartPointer } from '@/views/Weekly/charts/useChartPointer'
import WeeklyChartTooltip from '@/views/Weekly/charts/WeeklyChartTooltip.vue'

const props = defineProps<{ period: 'day' | 'week'; day: string }>()
const { t, locale } = useI18n()
const gradPrefix = `tok-${useId()}`
const { usage, state, reload } = useTokenUsage(props.period, toRef(props, 'day'))
type ChartKind = 'line' | 'bar' | 'pie'
const kind = ref<ChartKind>('line')
const kindOptions = computed<DgSelectOption<ChartKind>[]>(() => [
  { value: 'line', label: t('tokenUsage.line') },
  { value: 'bar', label: t('tokenUsage.bar') },
  { value: 'pie', label: t('tokenUsage.pie') },
])
const buckets = computed(() => usage.value?.buckets ?? [])
const totals = computed(() => tokenUsageTotals(buckets.value))
const input = computed(() => tokenUsagePoints(buckets.value, 'inputTokens'))
const output = computed(() => tokenUsagePoints(buckets.value, 'outputTokens'))
const max = computed(() => Math.max(1, ...buckets.value.map(b => Math.max(b.inputTokens, b.outputTokens))))
const path = (points: { x: number; y: number }[]) => points.map(p => `${p.x},${p.y}`).join(' ')
const areaPath = (points: { x: number; y: number }[]) => {
  if (points.length === 0) return ''
  const first = points[0]!
  const last = points[points.length - 1]!
  return `M${first.x},210 ` + points.map(p => `L${p.x},${p.y}`).join(' ') + ` L${last.x},210 Z`
}
const number = (value: number) => new Intl.NumberFormat(locale.value).format(value)
const formatter = computed(() => tokenUsageFormatter(locale.value, props.period, usage.value?.timeZone))
const label = (ts: number) => formatter.value.format(new Date(ts * 1000))
const { container, hovered, x: tipX, y: tipY, move, leave } = useChartPointer<number>()
const selected = computed(() => hovered.value === null ? null : buckets.value[hovered.value] ?? null)
const selectedInput = computed(() => hovered.value === null ? null : input.value[hovered.value] ?? null)
const selectedOutput = computed(() => hovered.value === null ? null : output.value[hovered.value] ?? null)
const selectedX = computed(() => hovered.value === null ? 0 : input.value[hovered.value]?.x ?? 0)
const intervalFormatter = computed(() => new Intl.DateTimeFormat(locale.value, {
  month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  timeZoneName: 'short', timeZone: safeTimeZone(usage.value?.timeZone),
}))
const intervalLabel = computed(() => {
  const bucket = selected.value
  if (!bucket) return ''
  return intervalFormatter.value.format(new Date(bucket.startTs * 1000)) + ' – ' +
    intervalFormatter.value.format(new Date(bucket.endTs * 1000))
})
function hover(event: PointerEvent): void {
  const point = svgPoint(event)
  const index = point ? tokenUsageHoverIndex(point.x, point.y, buckets.value.length) : null
  if (index === null) leave()
  else move(event, index)
}
watch([kind, buckets], leave)

const RING_R = 68
const RING_C = 2 * Math.PI * RING_R
const ringInput = computed(() => {
  const total = totals.value.input + totals.value.output
  return total > 0 ? (totals.value.input / total) * RING_C : 0
})
const ringOutput = computed(() => Math.max(0, RING_C - ringInput.value))
const inputShare = computed(() => {
  const total = totals.value.input + totals.value.output
  return total > 0 ? Math.round((totals.value.input / total) * 100) : 0
})
</script>

<template>
  <section class="token-card">
    <header>
      <div><h2>{{ t('tokenUsage.title') }}</h2><p>{{ t('tokenUsage.scope') }}</p></div>
      <DgSelect v-model="kind" class="kind-select" size="sm" :options="kindOptions" :aria-label="t('tokenUsage.chartType')" />
    </header>
    <p v-if="state === 'loading' && !usage">{{ t('tokenUsage.loading') }}</p>
    <div v-else-if="state === 'failed'"><p>{{ t('tokenUsage.failed') }}</p><button type="button" class="dg-button dg-button--sm" @click="reload">{{ t('common.action.retry') }}</button></div>
    <template v-else>
      <div class="totals">
        <strong>{{ number(totals.input + totals.output) }}</strong>
        <span class="input">{{ t('tokenUsage.input') }} · {{ number(totals.input) }}</span>
        <span class="output">{{ t('tokenUsage.output') }} · {{ number(totals.output) }}</span>
        <span>{{ t('tokenUsage.calls', { count: number(totals.calls) }) }}</span>
      </div>
      <p v-if="totals.unknown">{{ t('tokenUsage.unknown', { count: number(totals.unknown) }) }}</p>
      <p v-if="!totals.calls" class="empty">{{ t('tokenUsage.empty') }}</p>
      <p v-else-if="totals.input + totals.output === 0" class="empty">{{ t('tokenUsage.noReported') }}</p>
      <div v-else-if="kind === 'pie'" class="pie-row">
        <div class="pie-ring" role="img" :aria-label="t('tokenUsage.pieLabel', { input: number(totals.input), output: number(totals.output) })">
          <svg viewBox="0 0 180 180" aria-hidden="true">
            <circle cx="90" cy="90" :r="RING_R" class="pie-ring__track" />
            <circle
              cx="90"
              cy="90"
              :r="RING_R"
              class="pie-ring__seg pie-ring__seg--input"
              :stroke-dasharray="`${ringInput} ${RING_C}`"
            />
            <circle
              cx="90"
              cy="90"
              :r="RING_R"
              class="pie-ring__seg pie-ring__seg--output"
              :stroke-dasharray="`${ringOutput} ${RING_C}`"
              :stroke-dashoffset="-ringInput"
            />
          </svg>
          <div class="pie-ring__center">
            <strong>{{ number(totals.input + totals.output) }}</strong>
            <span>{{ inputShare }}% / {{ 100 - inputShare }}%</span>
          </div>
        </div>
        <p>{{ t('tokenUsage.composition') }}</p>
      </div>
      <div v-else ref="container" class="chart-area">
        <svg class="chart" viewBox="0 0 780 250" role="img" :aria-label="t('tokenUsage.title')" @pointermove="hover" @pointerleave="leave" @pointercancel="leave">
          <defs>
            <linearGradient :id="`${gradPrefix}-in`" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" class="input-stop-top" />
              <stop offset="100%" class="input-stop-bottom" />
            </linearGradient>
            <linearGradient :id="`${gradPrefix}-out`" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" class="output-stop-top" />
              <stop offset="100%" class="output-stop-bottom" />
            </linearGradient>
          </defs>
          <g v-for="tick in [0, 0.5, 1]" :key="tick">
            <line x1="52" x2="748" :y1="210 - tick * 170" :y2="210 - tick * 170" class="grid" />
            <text x="44" :y="214 - tick * 170" text-anchor="end">{{ number(Math.round(max * tick)) }}</text>
          </g>
          <template v-if="kind === 'line'">
            <path :d="areaPath(input)" :fill="`url(#${gradPrefix}-in)`" />
            <path :d="areaPath(output)" :fill="`url(#${gradPrefix}-out)`" />
            <polyline :points="path(input)" class="input-line" />
            <polyline :points="path(output)" class="output-line" />
          </template>
          <g v-for="(bucket, i) in buckets" :key="bucket.startTs">
            <template v-if="kind === 'bar'">
              <rect :x="input[i]!.x - 696 / buckets.length * 0.32" :y="input[i]!.y" :width="696 / buckets.length * 0.28" :height="input[i]!.height" class="input-fill" rx="2" />
              <rect :x="output[i]!.x + 696 / buckets.length * 0.04" :y="output[i]!.y" :width="696 / buckets.length * 0.28" :height="output[i]!.height" class="output-fill" rx="2" />
            </template>
            <template v-else>
              <circle :cx="input[i]!.x" :cy="input[i]!.y" r="4" class="input-fill" />
              <circle :cx="output[i]!.x" :cy="output[i]!.y" r="4" class="output-fill" />
            </template>
            <text v-if="period === 'week' || i % 4 === 0 || i === buckets.length - 1" :x="input[i]!.x" y="235" text-anchor="middle">{{ label(bucket.startTs) }}</text>
          </g>
          <g v-if="selected" class="hover-marker">
            <line :x1="selectedX" :x2="selectedX" y1="40" y2="210" class="hover-line" vector-effect="non-scaling-stroke" />
            <circle v-if="kind === 'line' && selectedInput" :cx="selectedX" :cy="selectedInput.y" r="6" class="input-fill hover-point" />
            <circle v-if="kind === 'line' && selectedOutput" :cx="selectedX" :cy="selectedOutput.y" r="6" class="output-fill hover-point" />
          </g>
        </svg>
        <WeeklyChartTooltip :visible="selected !== null" :x="tipX" :y="tipY">
          <template v-if="selected">
            <b>{{ intervalLabel }}</b>
            <span class="input">{{ t('tokenUsage.input') }} · {{ number(selected.inputTokens) }}</span>
            <span class="output">{{ t('tokenUsage.output') }} · {{ number(selected.outputTokens) }}</span>
            <span>{{ t('tokenUsage.total') }} · {{ number(selected.inputTokens + selected.outputTokens) }}</span>
            <span>{{ t('tokenUsage.calls', { count: number(selected.calls) }) }}</span>
            <span v-if="selected.unknownCalls">{{ t('tokenUsage.unknown', { count: number(selected.unknownCalls) }) }}</span>
          </template>
        </WeeklyChartTooltip>
      </div>
      <p class="note">{{ t('tokenUsage.note') }}</p>
    </template>
  </section>
</template>

<style scoped>
.token-card {
  background: var(--dg-wk-card-fill);
  border: 1px solid var(--dg-wk-card-border);
  border-radius: var(--dg-card-radius);
  box-shadow: var(--dg-card-shadow);
  padding: 20px 22px;
  color: var(--dg-text-primary);
}
header { display: flex; justify-content: space-between; align-items: start; gap: 16px; }
h2 {
  color: var(--dg-wk-title);
  font-family: var(--dg-font-page-title);
  font-size: 20px;
  font-weight: var(--dg-page-title-weight);
  line-height: 1.25;
}
p { color: var(--dg-text-muted); font-size: 12px; line-height: 1.6; margin-top: 6px; }
.kind-select { flex: none; width: 132px; }
.totals { display: flex; flex-wrap: wrap; align-items: baseline; gap: 18px; margin-top: 20px; font-size: 12px; font-variant-numeric: tabular-nums; }
.totals strong { font-size: 28px; font-weight: 600; }
.input { color: var(--dg-accent-text); } .output { color: var(--dg-token-output); }
.chart-area { position: relative; min-width: 0; }
.hover-marker { pointer-events: none; }
.hover-line { stroke: var(--dg-text-tertiary); stroke-width: 1; stroke-dasharray: 3 3; }
.hover-point { stroke: var(--dg-surface); stroke-width: 2; }
.chart { width: 100%; min-height: 180px; margin-top: 12px; overflow: visible; }
.chart text { fill: var(--dg-text-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.grid { stroke: var(--dg-wk-divider); }
.input-stop-top { stop-color: var(--dg-accent-text); stop-opacity: 0.2; }
.input-stop-bottom { stop-color: var(--dg-accent-text); stop-opacity: 0; }
.output-stop-top { stop-color: var(--dg-token-output); stop-opacity: 0.16; }
.output-stop-bottom { stop-color: var(--dg-token-output); stop-opacity: 0; }
.input-line, .output-line { fill: none; stroke-width: 2.25; stroke-linejoin: round; stroke-linecap: round; }
.input-line { stroke: var(--dg-accent-text); } .output-line { stroke: var(--dg-token-output); stroke-dasharray: 5 4; }
.input-fill { fill: var(--dg-accent-text); } .output-fill { fill: var(--dg-token-output); }
.pie-row { display: flex; align-items: center; justify-content: center; gap: 28px; padding: 24px 16px; }
.pie-ring { position: relative; width: 176px; height: 176px; flex-shrink: 0; }
.pie-ring svg { display: block; width: 100%; height: 100%; transform: rotate(-90deg); }
.pie-ring__track { fill: none; stroke: var(--dg-wk-divider); stroke-width: 18; }
.pie-ring__seg { fill: none; stroke-width: 18; transition: stroke-dasharray var(--dg-motion-slow) var(--dg-ease-glide); }
.pie-ring__seg--input { stroke: var(--dg-accent-text); }
.pie-ring__seg--output { stroke: var(--dg-token-output); }
.pie-ring__center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.pie-ring__center strong { font-size: 16px; font-weight: 650; color: var(--dg-text-primary); }
.pie-ring__center span { font-size: 11px; color: var(--dg-text-muted); }
.empty { padding: 32px 0; text-align: center; }
.note { font-size: 11px; }
@media (max-width: 600px) { .token-card { padding: 16px; } .pie-row { flex-direction: column; } }
</style>
