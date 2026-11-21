import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { useDailyStore } from '@/stores/daily'
import { useDayGoalStore } from '@/stores/dayGoal'
import { usePlanStore } from '@/stores/plan'
import { useTimelineStore } from '@/stores/timeline'
import { useUIVisibilityStore } from '@/stores/uiVisibility'
import { nativeFixtureWindow } from './nativeFixtureWindow'

test('hidden timeline, daily, goal and plan events refresh affected data once on reveal', async () => {
  const previous = { window: globalThis.window, document: globalThis.document }
  const listeners = new Map<string, Set<(payload: unknown) => void>>()
  const emit = (name: string, payload: unknown) => listeners.get(name)?.forEach((listener) => listener(payload))
  const reads: string[] = []
  const day = '2026-11-21'
  const weekDay = '2026-11-20'
  globalThis.window = nativeFixtureWindow(Object.assign(new EventTarget(), {
    go: { app: { Backend: {
      GetUIVisibility: async () => ({ visible: true }),
      GetDayContext: async (requested: string) => {
        reads.push(`context:${requested}`)
        return { day, standupDay: day, weekStart: '2026-11-16', dayStartTs: 1, dayEndTs: 2 }
      },
      GetTimelineDay: async () => ({ day, cards: [], categories: [], failures: [], processingRanges: [] }),
      GetCapabilities: async () => ({ canWrite: true, isCaptureOwner: true, features: ['timeline'] }),
      GetDailyRecap: async () => { reads.push('recap'); return { standupDay: day } },
      GetJournalDay: async () => { reads.push('journal'); return { day } },
      SaveJournalDay: async () => {},
      GetDayGoal: async () => { reads.push('goal'); return { day } },
      SaveDayGoal: async () => {},
      GetPlanDay: async (requested: string) => {
        reads.push(`plan:${requested}`)
        return { day: requested, blocks: [] }
      },
    } } },
    runtime: { EventsOnMultiple: (name: string, handler: (payload: unknown) => void) => {
      let handlers = listeners.get(name)
      if (!handlers) listeners.set(name, handlers = new Set())
      handlers.add(handler)
      return () => handlers.delete(handler)
    } },
  }) as unknown as Window & typeof globalThis)
  globalThis.document = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
  setActivePinia(createPinia())
  const visibility = useUIVisibilityStore()
  const timeline = useTimelineStore()
  const daily = useDailyStore()
  const goal = useDayGoalStore()
  const plan = usePlanStore()
  const flush = () => new Promise<void>((done) => setImmediate(done))
  try {
    visibility.start()
    timeline.startEvents(() => true)
    daily.startEvents()
    goal.startListening()
    plan.startListening()
    await Promise.all([timeline.load(), daily.load(day), goal.load(day), plan.load(day), plan.loadWeek([weekDay])])
    assert.equal(daily.context?.day, day)
    reads.length = 0

    emit('ui:visibility-changed', { visible: false })
    for (let i = 0; i < 3; i++) {
      emit('timeline:updated', { day })
      emit('journal:updated', { day })
      emit('recap:updated', { standupDay: day })
      emit('goal:updated', { day })
      emit('plan:updated', { day: weekDay })
      emit('plan:updated', { day: '2026-11-01' })
    }
    await flush()
    assert.deepEqual(reads, [])

    emit('ui:visibility-changed', { visible: true })
    await flush()
    assert.deepEqual(reads.sort(), [
      'context:', `context:${day}`, 'goal', 'journal', `plan:${weekDay}`, `plan:${day}`, 'recap',
    ])

    reads.length = 0
    emit('ui:visibility-changed', { visible: false })
    emit('journal:updated', { day })
    emit('journal:updated', { day })
    emit('ui:visibility-changed', { visible: true })
    await flush()
    assert.deepEqual(reads.sort(), ['context:', 'journal'])
  } finally {
    timeline.stopListening()
    daily.stopListening()
    goal.stopListening()
    plan.stopListening()
    visibility.stop()
    Object.assign(globalThis, previous)
  }
})
