import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import type { DayContextDTO, TimelineDayDTO } from '../src/api/dto'
import { usePlanStore } from '../src/stores/plan'
import { useTimelineStore } from '../src/stores/timeline'

/*
 * A date switch must not tear the day view down: the previous day stays on
 * screen (no loading state) until the next one arrives, then context and day
 * change together. A failed switch surfaces as a failure for the new date,
 * never as the old day posing under the new one.
 */

function contextOf(dayKey: string, start: number): DayContextDTO {
  return {
    day: dayKey, standupDay: dayKey, weekStart: '2026-09-14',
    dayStartTs: start, dayEndTs: start + 86400, nowTs: start, timeZone: 'Asia/Shanghai', dayBoundaryHour: 4,
  }
}

function dayOf(context: DayContextDTO): TimelineDayDTO {
  return {
    day: context.day, dayStartTs: context.dayStartTs, dayEndTs: context.dayEndTs,
    cards: [], categories: [], trackedMinutes: 0, idleMinutes: 0, failures: [], processingRanges: [], generatedAtTs: 1,
  }
}

const capabilities = { canWrite: true, isCaptureOwner: true, features: ['timeline'], appVersion: 'test', apiRevision: 1 }

function installBackend(backend: Record<string, unknown>): () => void {
  const previous = globalThis.window
  globalThis.window = { go: { app: { Backend: backend } } } as unknown as Window & typeof globalThis
  return () => { globalThis.window = previous }
}

test('a day switch keeps the previous day rendered and commits the next one atomically', async () => {
  const first = contextOf('2026-09-15', 100)
  const second = contextOf('2026-09-16', 100 + 86400)
  let releaseDay: (() => void) | undefined
  const restore = installBackend({
    GetDayContext: async (requested: string) => (requested === second.day ? second : first),
    GetTimelineDay: async (key: string) => {
      if (key === second.day) await new Promise<void>((resolve) => { releaseDay = resolve })
      return dayOf(key === second.day ? second : first)
    },
    GetCapabilities: async () => capabilities,
  })
  try {
    setActivePinia(createPinia())
    const store = useTimelineStore()
    await store.load(first.day)
    assert.equal(store.day?.day, first.day)

    const switching = store.load(second.day, { keep: true })
    await new Promise((resolve) => setTimeout(resolve, 0))
    // Mid-switch: nothing unmounts, and context has not run ahead of the day.
    assert.equal(store.loading, false)
    assert.equal(store.switching, true)
    assert.equal(store.state, 'empty')
    assert.equal(store.context?.day, first.day)
    assert.equal(store.day?.day, first.day)

    releaseDay?.()
    await switching
    assert.equal(store.switching, false)
    assert.equal(store.context?.day, second.day)
    assert.equal(store.day?.day, second.day)
  } finally {
    restore()
  }
})

test('a failed day switch shows the failure for the new date', async () => {
  const first = contextOf('2026-09-15', 100)
  const second = contextOf('2026-09-16', 100 + 86400)
  const restore = installBackend({
    GetDayContext: async (requested: string) => (requested === second.day ? second : first),
    GetTimelineDay: async (key: string) => {
      if (key === second.day) throw new Error('database_error')
      return dayOf(first)
    },
    GetCapabilities: async () => capabilities,
  })
  try {
    setActivePinia(createPinia())
    const store = useTimelineStore()
    await store.load(first.day)
    await store.load(second.day, { keep: true })
    assert.equal(store.day, null)
    assert.equal(store.context?.day, second.day, 'retry must target the new date')
    assert.equal(store.state, 'failure')
    assert.equal(store.switching, false)
  } finally {
    restore()
  }
})

test('switching the plan day keeps the old list until the new one arrives', async () => {
  const block = {
    id: 1, day: '2026-09-15', start: '09:00', end: '10:00', startTs: 1, endTs: 2, title: 'Anonymous',
    categoryId: '', categoryName: '', colorHex: '', status: 'planned', remind: false, matchedMinutes: 0, distractionMinutes: 0,
  }
  let release: (() => void) | undefined
  const restore = installBackend({
    GetPlanDay: async (key: string) => {
      if (key === '2026-09-16') {
        await new Promise<void>((resolve) => { release = resolve })
        return { day: key, blocks: [] }
      }
      return { day: key, blocks: [block] }
    },
  })
  try {
    setActivePinia(createPinia())
    const plan = usePlanStore()
    await plan.load('2026-09-15')
    assert.equal(plan.blocks.length, 1)

    const next = plan.load('2026-09-16')
    await new Promise((resolve) => setTimeout(resolve, 0))
    assert.equal(plan.blocks.length, 1, 'the panel must not blink empty')
    assert.equal(plan.day, '2026-09-15')

    release?.()
    await next
    assert.equal(plan.day, '2026-09-16')
    assert.equal(plan.blocks.length, 0)
  } finally {
    restore()
  }
})
