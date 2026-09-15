import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import type { DayContextDTO, TimelineDayDTO } from '../src/api/dto'
import { useTimelineStore } from '../src/stores/timeline'

const context: DayContextDTO = {
  day: '2026-09-15',
  standupDay: '2026-09-15',
  weekStart: '2026-09-14',
  dayStartTs: 1,
  dayEndTs: 2,
  nowTs: 1,
  timeZone: 'Asia/Shanghai',
  dayBoundaryHour: 4,
}

const day: TimelineDayDTO = {
  day: context.day,
  dayStartTs: context.dayStartTs,
  dayEndTs: context.dayEndTs,
  cards: [],
  categories: [],
  trackedMinutes: 0,
  idleMinutes: 0,
  failures: [],
  processingRanges: [],
  generatedAtTs: 1,
}

test('silent window-return refresh keeps the loaded timeline visible', async () => {
  const previousWindow = globalThis.window
  let releaseRefresh: (() => void) | undefined
  let contextCalls = 0
  globalThis.window = {
    go: {
      app: {
        Backend: {
          GetDayContext: async () => {
            contextCalls += 1
            if (contextCalls > 1) {
              await new Promise<void>((resolve) => { releaseRefresh = resolve })
            }
            return context
          },
          GetTimelineDay: async () => day,
          GetCapabilities: async () => ({
            canWrite: true,
            isCaptureOwner: true,
            features: ['timeline'],
            appVersion: 'test',
            apiRevision: 1,
          }),
        },
      },
    },
  } as unknown as Window & typeof globalThis

  try {
    setActivePinia(createPinia())
    const store = useTimelineStore()
    await store.load()
    assert.equal(store.state, 'empty')

    const refresh = store.load('', { silent: true })
    await Promise.resolve()
    assert.equal(store.loading, false)
    assert.equal(store.state, 'empty')

    releaseRefresh?.()
    await refresh
  } finally {
    globalThis.window = previousWindow
  }
})
