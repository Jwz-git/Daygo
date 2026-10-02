<script setup lang="ts">
import { computed, ref, toRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTokenUsage } from '@/stores/tokenUsage'
import { tokenUsageFormatter, tokenUsageHoverIndex, tokenUsagePoints, tokenUsageTotals } from '@/lib/tokenUsageChart'

import { safeTimeZone } from '@/lib/timeZone'
import { svgPoint, useChartPointer } from '@/views/Weekly/charts/useChartPointer'
import WeeklyChartTooltip from '@/views/Weekly/charts/WeeklyChartTooltip.vue'

const props = defineProps<{ period: 'day' | 'week'; day: string }>()
const { t, locale } = useI18n()
const { usage, state, reload } = useTokenUsage(props.period, toRef(props, 'day'))
const kind = ref<'line' | 'bar' | 'pie'>('line')
const buckets = computed(() => usage.value?.buckets ?? [])
const totals = computed(() => tokenUsageTotals(buckets.value))
const input = computed(() => tokenUsagePoints(buckets.value, 'inputTokens'))
const output = computed(() => tokenUsagePoints(buckets.value, 'outputTokens'))
const max = computed(() => Math.max(1, ...buckets.value.map(b => Math.max(b.inputTokens, b.outputTokens))))
const path = (points: { x: number; y: number }[]) => points.map(p => `${p.x},${p.y}`).join(' ')
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

const pie = computed(() => {
  const total = totals.value.input + totals.value.output
  const angle = total ? totals.value.input / total * 360 : 0
  return { background: `conic-gradient(var(--dg-accent-text) 0deg ${angle}deg, #6b8e9a ${angle}deg 360deg)` }
})
</script>

<template>
  <section class="token-card">
    <header>
      <div><h2>{{ t('tokenUsage.title') }}</h2><p>{{ t('tokenUsage.scope') }}</p></div>
      <select v-model="kind" :aria-label="t('tokenUsage.chartType')">
        <option value="line">{{ t('tokenUsage.line') }}</option>
        <option value="bar">{{ t('tokenUsage.bar') }}</option>
        <option value="pie">{{ t('tokenUsage.pie') }}</option>
      </select>
    </header>
    <p v-if="state === 'loading' && !usage">{{ t('tokenUsage.loading') }}</p>
    <div v-else-if="state === 'failed'"><p>{{ t('tokenUsage.failed') }}</p><button @click="reload">{{ t('common.action.retry') }}</button></div>
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
        <div class="pie" :style="pie" role="img" :aria-label="t('tokenUsage.pieLabel', { input: number(totals.input), output: number(totals.output) })" />
        <p>{{ t('tokenUsage.composition') }}</p>
      </div>
      <div v-else ref="container" class="chart-area">
        <svg class="chart" viewBox="0 0 780 250" role="img" :aria-label="t('tokenUsage.title')" @pointermove="hover" @pointerleave="leave" @pointercancel="leave">
          <g v-for="tick in [0, 0.5, 1]" :key="tick">
            <line x1="52" x2="748" :y1="210 - tick * 170" :y2="210 - tick * 170" class="grid" />
            <text x="44" :y="214 - tick * 170" text-anchor="end">{{ number(Math.round(max * tick)) }}</text>
          </g>
          <template v-if="kind === 'line'">
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
.token-card { background: var(--dg-wk-card-fill); border: 1px solid var(--dg-wk-divider); border-radius: 4px; padding: 24px; color: var(--dg-text-primary); }
header { display: flex; justify-content: space-between; align-items: start; gap: 16px; }
h2 { font-family: var(--dg-font-serif); font-size: 23px; font-weight: 400; }
p { color: var(--dg-text-muted); font-size: 12px; line-height: 1.6; margin-top: 8px; }
select { max-width: 150px; padding: 6px 10px; background: var(--dg-wk-card-fill); color: var(--dg-text-primary); border: 1px solid var(--dg-wk-divider); border-radius: 8px; }
.totals { display: flex; flex-wrap: wrap; align-items: baseline; gap: 18px; margin-top: 24px; font-size: 12px; }
.totals strong { font-size: 30px; font-weight: 500; }
.input { color: var(--dg-accent-text); } .output { color: #6b8e9a; }
.chart-area { position: relative; min-width: 0; }
.hover-marker { pointer-events: none; }
.hover-line { stroke: var(--dg-text-tertiary); stroke-width: 1; }
.hover-point { stroke: var(--dg-wk-card-fill); stroke-width: 2; }
.chart { width: 100%; min-height: 180px; margin-top: 12px; overflow: visible; }
.chart text { fill: var(--dg-text-muted); font-size: 11px; }
.grid { stroke: var(--dg-wk-divider); }
.input-line, .output-line { fill: none; stroke-width: 2.5; stroke-linejoin: round; }
.input-line { stroke: var(--dg-accent-text); } .output-line { stroke: #6b8e9a; stroke-dasharray: 5 4; }
.input-fill { fill: var(--dg-accent-text); } .output-fill { fill: #6b8e9a; }
.pie-row { display: flex; align-items: center; justify-content: center; gap: 30px; padding: 28px; }
.pie { width: 180px; height: 180px; border-radius: 50%; flex-shrink: 0; }
.empty { padding: 32px 0; text-align: center; }
.note { font-size: 11px; }
@media (max-width: 600px) { .token-card { padding: 16px; } .pie-row { flex-direction: column; } }
</style>
