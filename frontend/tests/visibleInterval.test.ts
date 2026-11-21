import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { effectScope } from 'vue'

import { useVisibleInterval } from '@/lib/visibleInterval'
import { useUIVisibilityStore } from '@/stores/uiVisibility'
import { nativeFixtureWindow } from './nativeFixtureWindow'

test('visible interval pauses while the host is hidden and refreshes once on reveal', async () => {
  const previous = { window: globalThis.window, document: globalThis.document }
  const timers = new Map<number, () => void>()
  let next = 0
  let ticks = 0
  let setVisibility: (payload: unknown) => void = () => undefined
  globalThis.window = nativeFixtureWindow(Object.assign(new EventTarget(), {
    go: { app: { Backend: { GetUIVisibility: async () => ({ visible: true }) } } },
    runtime: { EventsOnMultiple: (_name: string, handler: (payload: unknown) => void) => {
      setVisibility = handler
      return () => { setVisibility = () => undefined }
    } },
    setInterval: (callback: () => void) => { timers.set(++next, callback); return next },
    clearInterval: (id: number) => { timers.delete(id) },
  }) as unknown as Window & typeof globalThis)
  globalThis.document = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
  setActivePinia(createPinia())
  const visibility = useUIVisibilityStore()
  const scope = effectScope()
  try {
    visibility.start()
    await new Promise<void>((done) => setImmediate(done))
    scope.run(() => useVisibleInterval(() => { ticks++ }, 30_000))
    assert.equal(ticks, 1)
    assert.equal(timers.size, 1)
    setVisibility({ visible: false })
    assert.equal(timers.size, 0)
    setVisibility({ visible: true })
    assert.equal(ticks, 2)
    assert.equal(timers.size, 1)
  } finally {
    scope.stop()
    visibility.stop()
    Object.assign(globalThis, previous)
  }
  assert.equal(timers.size, 0)
})
