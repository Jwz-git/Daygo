import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'

import type { DayGoalDTO } from '@/api/dto'
import { useDayGoalStore } from '@/stores/dayGoal'
import { nativeFixtureWindow } from './nativeFixtureWindow'

test('timeline goal refresh only reads goals and respects the visible day', async () => {
  const day = '2026-11-21'
  let saved: DayGoalDTO = {
    day, exists: true, focusTargetMinutes: 60, distractionLimitMinutes: 0,
    isSkipped: false, focusCategories: [], distractionCategories: [],
  }
  const listeners = new Map<string, (...payload: unknown[]) => void>()
  const reads: string[] = []
  const previous = globalThis.window
  globalThis.window = nativeFixtureWindow({
    go: { app: { Backend: {
      GetDayGoal: async (selected: string) => { reads.push(selected); return { ...saved } },
      SaveDayGoal: async (goal: DayGoalDTO) => { saved = goal },
    } } },
    runtime: {
      EventsOnMultiple: (name: string, callback: (...payload: unknown[]) => void) => {
        listeners.set(name, callback)
        return () => listeners.delete(name)
      },
    },
  } as unknown as Window & typeof globalThis)

  setActivePinia(createPinia())
  const store = useDayGoalStore()
  try {
    store.startListening()
    assert.deepEqual([...listeners.keys()], ['goal:updated'])
    await store.load(day)
    assert.deepEqual(reads, [day])
    assert.equal(store.goal?.focusTargetMinutes, 60)
    listeners.get('goal:updated')?.({ day: '2026-11-20' })
    assert.deepEqual(reads, [day])
    listeners.get('goal:updated')?.({ day: '' })
    assert.deepEqual(reads, [day, day])
    await store.save({ ...saved, focusTargetMinutes: 90 })
    assert.equal(store.goal?.focusTargetMinutes, 90)
    assert.deepEqual(reads, [day, day, day])
  } finally {
    store.stopListening()
    globalThis.window = previous
  }
  assert.equal(listeners.size, 0)
})
