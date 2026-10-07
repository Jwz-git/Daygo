import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'

import type { PlanBlockDTO, PlanDayDTO } from '../src/api/dto'
import { usePlanStore, writeErrorOf } from '../src/stores/plan'
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

const planDay = (day: string, minutes: number): PlanDayDTO => ({
  day,
  blocks: [{
    id: 1, day, start: '09:00', end: '10:00', startTs: 100, endTs: 3700,
    title: 'Anonymous plan', notes: null, categoryId: null, categoryName: '', colorHex: '',
    status: 'planned', completedAtTs: null, remind: false,
    matchedMinutes: minutes, distractionMinutes: minutes / 2,
  } satisfies PlanBlockDTO],
})

async function withPlanBackend(
  query: (day: string) => Promise<PlanDayDTO>,
  run: (emit: (name: string, day?: string) => Promise<void>, listeners: Map<string, (...args: unknown[]) => void>) => Promise<void>,
): Promise<void> {
  const previousWindow = globalThis.window
  const listeners = new Map<string, (...args: unknown[]) => void>()
  globalThis.window = {
    go: { app: { Backend: { GetPlanDay: query } } },
    runtime: { EventsOnMultiple: (name: string, callback: (...args: unknown[]) => void) => {
      assert.equal(listeners.has(name), false, `duplicate listener: ${name}`)
      listeners.set(name, callback)
      return () => { listeners.delete(name) }
    } },
  } as unknown as Window & typeof globalThis
  try {
    setActivePinia(createPinia())
    await run(async (name, day) => {
      listeners.get(name)?.(day === undefined ? {} : { day })
      await new Promise<void>((resolve) => setImmediate(resolve))
    }, listeners)
  } finally {
    globalThis.window = previousWindow
  }
}

test('plan coverage follows timeline and goal events for the loaded day and week', async () => {
  let minutes = 0
  const queries: string[] = []
  await withPlanBackend(async (day) => {
    queries.push(day)
    return planDay(day, minutes)
  }, async (emit, listeners) => {
    const store = usePlanStore()
    await store.load('2026-10-05')
    await store.loadWeek(['2026-10-05', '2026-10-06'])
    store.startListening()
    store.startListening()
    assert.equal(listeners.size, 3)
    for (const event of ['timeline:updated', 'goal:updated', 'plan:updated']) {
      minutes += 10
      await emit(event, '2026-10-05')
      assert.equal(store.blocks[0]?.matchedMinutes, minutes)
      assert.equal(store.week['2026-10-05']?.[0]?.distractionMinutes, minutes / 2)
    }
    const previousQueries = queries.length
    await emit('timeline:updated', '2026-09-01')
    assert.equal(queries.length, previousQueries)
    minutes = 50
    await emit('goal:updated')
    assert.equal(store.blocks[0]?.matchedMinutes, 50)
    assert.equal(store.week['2026-10-06']?.[0]?.matchedMinutes, 50)
    store.stopListening()
    assert.equal(listeners.size, 0)
    await emit('timeline:updated', '2026-10-05')
    assert.equal(store.blocks[0]?.matchedMinutes, 50)
  })
})

test('an old week refresh cannot reinsert a day after switching weeks', async () => {
  let resolveOld: ((value: PlanDayDTO) => void) | undefined
  let hold = false
  await withPlanBackend(async (day) => hold && day === '2026-10-05'
    ? new Promise<PlanDayDTO>((resolve) => { resolveOld = resolve })
    : planDay(day, 10), async (emit) => {
    const store = usePlanStore()
    await store.loadWeek(['2026-10-05'])
    store.startListening()
    hold = true
    await emit('plan:updated', '2026-10-05')
    await store.loadWeek(['2026-10-12'])
    resolveOld?.(planDay('2026-10-05', 0))
    await new Promise<void>((resolve) => setImmediate(resolve))
    assert.deepEqual(Object.keys(store.week), ['2026-10-12'])
    store.stopListening()
  })
})

test('a newer event wins over an initial week query that finishes late', async () => {
  let resolveInitial: ((value: PlanDayDTO) => void) | undefined
  let calls = 0
  await withPlanBackend(async (day) => ++calls === 1
    ? new Promise<PlanDayDTO>((resolve) => { resolveInitial = resolve })
    : planDay(day, 20), async (emit) => {
    const store = usePlanStore()
    store.startListening()
    const initial = store.loadWeek(['2026-10-05'])
    await emit('timeline:updated', '2026-10-05')
    resolveInitial?.(planDay('2026-10-05', 0))
    await initial
    assert.equal(store.week['2026-10-05']?.[0]?.matchedMinutes, 20)
    store.stopListening()
  })
})
