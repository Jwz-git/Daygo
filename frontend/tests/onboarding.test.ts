import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import { useOnboardingStore } from '../src/stores/onboarding'

/*
 * The first-run guide covers the whole app, so the gate errs toward "done":
 * it shows only on the writer instance with the setting unset, and every
 * failure path lets the user straight in.
 */

function settings(onboardingCompleted: boolean) {
  return {
    capture: { intervalSeconds: 10, captureHeight: 1080 }, privacy: { blockedApplicationIds: [] },
    storage: { recordingsLimitBytes: 0 }, appearance: { theme: 'system', language: '' },
    llm: { showTokenUsage: false }, chat: { memory: '', editMode: 'readonly' },
    notifications: { journalReminderEnabled: false, journalReminderTime: '18:00' },
    system: { launchAtLogin: false, showDockIcon: true, agentEditsEnabled: false, testToolsEnabled: false, onboardingCompleted },
    telemetry: { analyticsOptIn: false, crashReportingOptIn: false },
  }
}

function capabilities(canWrite: boolean) {
  return { canWrite, isCaptureOwner: canWrite, features: [], appVersion: 'dev', apiRevision: 1 }
}

async function statusWith(backend: Record<string, unknown>): Promise<string> {
  const previous = globalThis.window
  globalThis.window = { go: { app: { Backend: backend } } } as unknown as typeof globalThis.window
  try {
    setActivePinia(createPinia())
    const store = useOnboardingStore()
    assert.equal(store.status, 'unknown')
    await store.initialize()
    return store.status
  } finally {
    globalThis.window = previous
  }
}

test('a writer with the setting unset gets the guide', async () => {
  assert.equal(await statusWith({ GetSettings: async () => settings(false), GetCapabilities: async () => capabilities(true) }), 'pending')
})

test('a completed guide, a read-only instance or a failed read skip it', async () => {
  assert.equal(await statusWith({ GetSettings: async () => settings(true), GetCapabilities: async () => capabilities(true) }), 'done')
  assert.equal(await statusWith({ GetSettings: async () => settings(false), GetCapabilities: async () => capabilities(false) }), 'done')
  assert.equal(await statusWith({
    GetSettings: async () => { throw new Error('database_error') },
    GetCapabilities: async () => capabilities(true),
  }), 'done')
})

test('completing writes the setting, and a failed write still lets the user in', async () => {
  const previous = globalThis.window
  const patches: unknown[] = []
  globalThis.window = { go: { app: { Backend: {
    UpdateSettings: async (patch: unknown) => {
      patches.push(patch)
      throw new Error('not_capture_owner')
    },
  } } } } as unknown as typeof globalThis.window
  try {
    setActivePinia(createPinia())
    const store = useOnboardingStore()
    await store.complete()
    assert.deepEqual(patches, [{ onboardingCompleted: true }])
    assert.equal(store.status, 'done')
  } finally {
    globalThis.window = previous
  }
})

test('a preview shows the guide without recording completion', async () => {
  const previous = globalThis.window
  const patches: unknown[] = []
  globalThis.window = { go: { app: { Backend: {
    UpdateSettings: async (patch: unknown) => { patches.push(patch); return settings(true) },
  } } } } as unknown as typeof globalThis.window
  try {
    setActivePinia(createPinia())
    const store = useOnboardingStore()
    store.startPreview()
    assert.equal(store.status, 'pending')
    assert.equal(store.preview, true)
    assert.equal(await store.markCompleted(), true)
    await store.complete()
    assert.deepEqual(patches, [], 'a preview must not write the completion setting')
    assert.equal(store.status, 'done')
    assert.equal(store.preview, false)
  } finally {
    globalThis.window = previous
  }
})
