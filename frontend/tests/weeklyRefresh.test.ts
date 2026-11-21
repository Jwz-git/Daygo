import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { useWeeklyStore } from '@/stores/weekly'
import { nativeFixtureWindow } from './nativeFixtureWindow'

test('weekly invalidations reload only the matching displayed or comparison week', async () => {
  const previous = globalThis.window
  const reads: string[] = []
  const week = '2026-11-16'
  const prior = '2026-11-09'
  let invalidate: (event: unknown) => void = () => undefined
  let releasePrevious: () => void = () => undefined
  const previousGate = new Promise<void>((done) => { releasePrevious = done })
  let gate = true
  globalThis.window = nativeFixtureWindow({
    go: { app: { Backend: {
      GetWeeklyDashboard: async (requested: string) => {
        reads.push(requested)
        if (requested === prior && gate) await previousGate
        return { weekStart: requested, weekStartTs: 1, weekEndTs: 2, trackedMinutes: 60, focusMinutes: 60, categories: [], days: [] }
      },
    } } },
    runtime: { EventsOnMultiple: (_name: string, callback: (event: unknown) => void) => {
      invalidate = callback
      return () => { invalidate = () => undefined }
    } },
  } as unknown as Window & typeof globalThis)
  const flush = () => new Promise<void>((done) => setImmediate(done))
  setActivePinia(createPinia())
  const store = useWeeklyStore()
  try {
    store.startEvents()
    await store.load(week)
    const workflow = store.charts?.workflow
    assert.ok(workflow)
    gate = false
    releasePrevious()
    await flush()
    assert.equal(store.charts?.workflow, workflow)

    invalidate({ day: '2026-10-30' })
    await flush()
    assert.deepEqual(reads, [week, prior])
    invalidate({ day: '2026-11-21' })
    assert.equal(store.loading, false)
    await flush()
    assert.deepEqual(reads, [week, prior, week])
    invalidate({ day: '2026-11-13' })
    await flush()
    assert.deepEqual(reads, [week, prior, week, prior])
  } finally {
    releasePrevious()
    store.stopListening()
    globalThis.window = previous
  }
})
