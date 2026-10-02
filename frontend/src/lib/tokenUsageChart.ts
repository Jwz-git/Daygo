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
