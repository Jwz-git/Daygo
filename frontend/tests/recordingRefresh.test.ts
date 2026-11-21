import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'

import { useRecordingStore } from '@/stores/recording'
import { useUIVisibilityStore } from '@/stores/uiVisibility'
import { nativeFixtureWindow } from './nativeFixtureWindow'

test('healthy frames update the timestamp without a binding read and hidden events coalesce', async () => {
  const previous = { window: globalThis.window, document: globalThis.document }
  const listeners = new Map<string, (payload: unknown) => void>()
  let reads = 0
  globalThis.window = nativeFixtureWindow(Object.assign(new EventTarget(), {
    go: { app: { Backend: {
      GetUIVisibility: async () => ({ visible: true }),
      GetRecordingState: async () => {
        reads++
        return { state: 'capturing', permission: 'granted', isCaptureOwner: true, lastFrameAtTs: 10 }
      },
    } } },
    runtime: { EventsOnMultiple: (name: string, handler: (payload: unknown) => void) => {
      listeners.set(name, handler)
      return () => { listeners.delete(name) }
    } },
  }) as unknown as Window & typeof globalThis)
  globalThis.document = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
  setActivePinia(createPinia())
  const visibility = useUIVisibilityStore()
  const recording = useRecordingStore()
  const flush = () => new Promise<void>((done) => setImmediate(done))
  try {
    visibility.start()
    recording.startListening()
    await flush()
    assert.equal(reads, 1)
    listeners.get('recording:state')?.({ state: 'capturing', lastFrameAtTs: 11 })
    await flush()
    assert.equal(reads, 1)
    assert.equal(recording.snapshot?.lastFrameAtTs, 11)

    listeners.get('ui:visibility-changed')?.({ visible: false })
    listeners.get('recording:state')?.({ state: 'paused' })
    listeners.get('recording:state')?.({ state: 'capturing', lastFrameAtTs: 12 })
    await flush()
    assert.equal(reads, 1)
    listeners.get('ui:visibility-changed')?.({ visible: true })
    await flush()
    assert.equal(reads, 2)

    // The desktop suppresses healthy frame events in the background.
    listeners.get('ui:visibility-changed')?.({ visible: false })
    await flush()
    assert.equal(reads, 2)
    listeners.get('ui:visibility-changed')?.({ visible: true })
    await flush()
    assert.equal(reads, 3)
  } finally {
    recording.stopListening()
    visibility.stop()
    Object.assign(globalThis, previous)
  }
})
