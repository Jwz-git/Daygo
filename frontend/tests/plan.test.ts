import assert from 'node:assert/strict'
import test from 'node:test'

import { writeErrorOf } from '../src/stores/plan'
import { planLanes, planPhase } from '../src/views/Timeline/planLayout'

// Binding errors cross Wails as "daygo:<code>: <message>"; the panel shows one
// message per class and never the raw text.
test('plan write errors map binding codes to the panel message classes', () => {
  assert.equal(writeErrorOf(new Error('daygo:invalid_argument: plan times must be HH:mm')), 'invalid')
  assert.equal(writeErrorOf('daygo:not_capture_owner: this instance is not the capture owner'), 'readonly')
  assert.equal(writeErrorOf(new Error('daygo:database_error: busy')), 'failed')
  assert.equal(writeErrorOf(new Error('network down')), 'failed')
})

const block = (id: number, start: number, end: number) => ({ id, startTs: start, endTs: end })
const lanesOf = (blocks: ReturnType<typeof block>[]) =>
  planLanes(blocks).map((entry) => [entry.block.id, entry.lane, entry.lanes])

// Overlapping plan blocks share their cluster's width side by side; a block
// that starts as another ends is adjacent, not overlapping.
test('plan lanes split only overlapping clusters', () => {
  assert.deepEqual(lanesOf([]), [])
  assert.deepEqual(lanesOf([block(1, 0, 60)]), [[1, 0, 1]])
  assert.deepEqual(lanesOf([block(1, 0, 60), block(2, 60, 120)]), [[1, 0, 1], [2, 0, 1]])
  // The user's day: 17:30–18:30 and 17:40–19:00 overlap.
  assert.deepEqual(lanesOf([block(2, 40, 120), block(1, 30, 90)]), [[1, 0, 2], [2, 1, 2]])
  // A chain where 3 reuses lane 0 once 1 ends; the cluster still needs 2 lanes.
  assert.deepEqual(
    lanesOf([block(1, 0, 30), block(2, 10, 60), block(3, 40, 70), block(4, 100, 110)]),
    [[1, 0, 2], [2, 1, 2], [3, 0, 2], [4, 0, 1]],
  )
  // Three at once.
  assert.deepEqual(
    lanesOf([block(1, 0, 60), block(2, 0, 60), block(3, 30, 90)]),
    [[1, 0, 3], [2, 1, 3], [3, 2, 3]],
  )
})

test('plan phase follows status first, then where now falls', () => {
  const span = { startTs: 100, endTs: 200 }
  assert.equal(planPhase({ ...span, status: 'planned' }, 99), 'upcoming')
  assert.equal(planPhase({ ...span, status: 'planned' }, 100), 'active')
  assert.equal(planPhase({ ...span, status: 'planned' }, 200), 'missed')
  assert.equal(planPhase({ ...span, status: 'done' }, 50), 'done')
  assert.equal(planPhase({ ...span, status: 'skipped' }, 150), 'skipped')
})
