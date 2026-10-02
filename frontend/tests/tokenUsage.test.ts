import test from 'node:test'
import assert from 'node:assert/strict'
import { tokenUsagePoints, tokenUsageTotals } from '../src/lib/tokenUsageChart'

test('token chart keeps unknown calls visible and input/output additive', () => {
  const buckets = [
    { startTs: 1, endTs: 2, inputTokens: 140, outputTokens: 20, calls: 2, unknownCalls: 1 },
    { startTs: 2, endTs: 3, inputTokens: 0, outputTokens: 0, calls: 0, unknownCalls: 0 },
  ]
  assert.deepEqual(tokenUsageTotals(buckets), { input: 140, output: 20, calls: 2, unknown: 1 })
  const points = tokenUsagePoints(buckets, 'inputTokens')
  assert.equal(points[0]?.y, 40)
  assert.equal(points[1]?.y, 210)
  assert.deepEqual(tokenUsagePoints([], 'outputTokens'), [])
})
