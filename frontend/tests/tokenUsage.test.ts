import test from 'node:test'
import assert from 'node:assert/strict'
import { tokenUsageFormatter, tokenUsageHoverIndex, tokenUsagePoints, tokenUsageTotals } from '../src/lib/tokenUsageChart'

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

test('token labels accept Go Local and malformed zones without changing the instant', () => {
  const at = new Date('2026-10-02T04:00:00Z')
  for (const period of ['day', 'week'] as const) {
    const options = period === 'day'
      ? { hour: '2-digit' as const, minute: '2-digit' as const }
      : { month: 'short' as const, day: 'numeric' as const }
    const expected = new Intl.DateTimeFormat('en', options).format(at)
    for (const zone of ['Local', 'Not/AZone', '', undefined]) {
      assert.equal(tokenUsageFormatter('en', period, zone).format(at), expected)
    }
    assert.equal(
      tokenUsageFormatter('en', period, 'Asia/Shanghai').format(at),
      new Intl.DateTimeFormat('en', { ...options, timeZone: 'Asia/Shanghai' }).format(at),
    )
  }
})

test('token hover selects the nearest time bucket only inside the plot', () => {
  assert.equal(tokenUsageHoverIndex(52, 40, 24), 0)
  assert.equal(tokenUsageHoverIndex(748, 210, 24), 23)
  assert.equal(tokenUsageHoverIndex(52 + 696 / 24, 100, 24), 1)
  assert.equal(tokenUsageHoverIndex(52 + 696 * 3.5 / 7, 100, 7), 3)
  for (const [x, y, count] of [[51, 100, 24], [749, 100, 24], [100, 39, 24], [100, 211, 24], [100, 100, 0], [NaN, 100, 24]]) {
    assert.equal(tokenUsageHoverIndex(x!, y!, count!), null)
  }
})
