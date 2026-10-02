<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTokenUsage } from '@/stores/tokenUsage'
import { tokenUsagePoints, tokenUsageTotals } from '@/lib/tokenUsageChart'

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
const label = (ts: number) => new Intl.DateTimeFormat(locale.value, {
  ...(props.period === 'day' ? { hour: '2-digit' as const, minute: '2-digit' as const } : { month: 'short' as const, day: 'numeric' as const }),
  timeZone: usage.value?.timeZone,
}).format(new Date(ts * 1000))
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
      <svg v-else class="chart" viewBox="0 0 780 250" role="img" :aria-label="t('tokenUsage.title')">
        <g v-for="tick in [0, 0.5, 1]" :key="tick">
          <line x1="52" x2="748" :y1="210 - tick * 170" :y2="210 - tick * 170" class="grid" />
          <text x="44" :y="214 - tick * 170" text-anchor="end">{{ number(Math.round(max * tick)) }}</text>
        </g>
        <template v-if="kind === 'line'">
          <polyline :points="path(input)" class="input-line" />
          <polyline :points="path(output)" class="output-line" />
        </template>
        <g v-for="(bucket, i) in buckets" :key="bucket.startTs">
          <title>{{ label(bucket.startTs) }} — {{ t('tokenUsage.input') }}: {{ number(bucket.inputTokens) }}; {{ t('tokenUsage.output') }}: {{ number(bucket.outputTokens) }}; {{ t('tokenUsage.calls', { count: number(bucket.calls) }) }}</title>
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
      </svg>
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
