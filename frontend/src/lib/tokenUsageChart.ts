import { safeTimeZone } from '@/lib/timeZone'
import type { TokenUsageBucket } from '@/api/tokenUsage'

export function tokenUsageTotals(buckets: TokenUsageBucket[]) {
  return buckets.reduce((sum, bucket) => ({
    input: sum.input + bucket.inputTokens,
    output: sum.output + bucket.outputTokens,
    calls: sum.calls + bucket.calls,
    unknown: sum.unknown + bucket.unknownCalls,
  }), { input: 0, output: 0, calls: 0, unknown: 0 })
}
export function tokenUsagePoints(buckets: TokenUsageBucket[], field: 'inputTokens' | 'outputTokens') {
  const max = Math.max(1, ...buckets.map(b => Math.max(b.inputTokens, b.outputTokens)))
  return buckets.map((b, i) => ({
    x: 52 + (i + 0.5) * 696 / Math.max(1, buckets.length),
    y: 210 - b[field] / max * 170,
    height: b[field] / max * 170,
  }))
}

// Go's Local and malformed identifiers must never reach Intl directly.
export function tokenUsageFormatter(locale: string, period: 'day' | 'week', zone: string | undefined) {
  return new Intl.DateTimeFormat(locale, {
    ...(period === 'day'
      ? { hour: '2-digit' as const, minute: '2-digit' as const }
      : { month: 'short' as const, day: 'numeric' as const }),
    timeZone: safeTimeZone(zone),
  })
}

// Both chart modes share the same horizontal time slots. Reject axis margins
// and empty data so hovering labels never selects an unrelated bucket.
export function tokenUsageHoverIndex(x: number, y: number, count: number): number | null {
  if (!Number.isFinite(x) || !Number.isFinite(y) || count <= 0 ||
      x < 52 || x > 748 || y < 40 || y > 210) return null
  return Math.min(count - 1, Math.floor((x - 52) / 696 * count))
}
