import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'

import type { DiagnosticsDTO } from '../src/api/diagnostics'
import { useDiagnosticsStore } from '../src/stores/diagnostics'

test('diagnostic refresh retains measured data on failure and retries without inventing counters', async () => {
  const previousWindow = globalThis.window
  const value = {
    databasePath: '', databaseBytes: 0, recordingsBytes: 0, lastCaptureAtTs: null,
    pendingBatches: 0, failedBatches: 0, nativeState: 'unavailable', captureOwnerPid: null,
    skippedCardsToday: 0, dbStatus: 'ok',
    storageHealth: { slowQueries: 2, queryErrors: 3, busyErrors: 1, maintenanceErrors: 0 },
  } satisfies DiagnosticsDTO
  let fail = false
  globalThis.window = { go: { app: { Backend: { GetDiagnostics: async () => {
    if (fail) throw new Error('daygo:database_error: anonymous failure')
    return value
  } } } } } as unknown as Window & typeof globalThis
  try {
    setActivePinia(createPinia())
    const store = useDiagnosticsStore()
    await store.load()
    assert.equal(store.diagnostics?.storageHealth?.queryErrors, 3)
    fail = true
    await store.load()
    assert.equal(store.loadFailed, true)
    assert.equal(store.loading, false)
    assert.equal(store.diagnostics?.storageHealth?.queryErrors, 3)
    fail = false
    await store.load()
    assert.equal(store.loadFailed, false)
  } finally {
    globalThis.window = previousWindow
  }
})
