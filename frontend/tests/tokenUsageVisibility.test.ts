import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import { useTokenUsageVisibilityStore } from '../src/stores/tokenUsageVisibility'

/*
 * The token-usage card is opt-in, so this store is the one place that decides
 * whether the daily and weekly reports render it. The cases below are the ones
 * a wrong default would break: an unreadable database must leave the card
 * hidden, and a failed write must not flip the switch on screen.
 */

type BackendStub = {
  GetSettings: () => Promise<unknown>
  UpdateSettings: (patch: Record<string, unknown>) => Promise<unknown>
}

function settingsWith(showTokenUsage: boolean) {
  return {
    capture: { intervalSeconds: 10, captureHeight: 1080 },
    privacy: { blockedApplicationIds: [] },
    storage: { recordingsLimitBytes: 0 },
    appearance: { theme: 'system', language: 'zh-CN' },
    llm: { showTokenUsage },
    chat: { memory: '', editMode: 'readonly' },
    notifications: { journalReminderEnabled: false, journalReminderTime: '18:00' },
    system: {
      launchAtLogin: false,
      showDockIcon: true,
      agentEditsEnabled: false,
      testToolsEnabled: false,
    },
  }
}

function stubWindow(backend: Partial<BackendStub>): () => void {
  const previous = globalThis.window
  globalThis.window = { go: { app: { Backend: backend } } } as unknown as typeof globalThis.window
  return () => { globalThis.window = previous }
}

test('the card stays hidden until a stored preference turns it on', async () => {
  const restore = stubWindow({
    GetSettings: async () => settingsWith(false),
  })
  try {
    setActivePinia(createPinia())
    const store = useTokenUsageVisibilityStore()

    assert.equal(store.showTokenUsage, false, 'the default must be off before any read')
    await store.initialize()
    assert.equal(store.showTokenUsage, false)
    assert.equal(store.loaded, true)
  } finally {
    restore()
  }
})

test('a stored preference of true reveals the card', async () => {
  const restore = stubWindow({
    GetSettings: async () => settingsWith(true),
  })
  try {
    setActivePinia(createPinia())
    const store = useTokenUsageVisibilityStore()

    await store.initialize()
    assert.equal(store.showTokenUsage, true)
  } finally {
    restore()
  }
})

test('an unreadable database leaves the card hidden', async () => {
  const restore = stubWindow({
    GetSettings: async () => { throw new Error('database_error') },
  })
  try {
    setActivePinia(createPinia())
    const store = useTokenUsageVisibilityStore()

    await store.initialize()
    assert.equal(store.showTokenUsage, false, 'a read failure must not reveal the card')
    assert.equal(store.loaded, true, 'the switch must leave its loading state')
  } finally {
    restore()
  }
})

test('toggling writes the setting and adopts the normalized value', async () => {
  const restore = stubWindow({
    GetSettings: async () => settingsWith(false),
    UpdateSettings: async (patch) => {
      assert.deepEqual(patch, { showTokenUsage: true })
      return settingsWith(true)
    },
  })
  try {
    setActivePinia(createPinia())
    const store = useTokenUsageVisibilityStore()

    assert.equal(await store.setShowTokenUsage(true), true)
    assert.equal(store.showTokenUsage, true)
  } finally {
    restore()
  }
})

test('a failed write reports failure and does not reveal the card', async () => {
  const restore = stubWindow({
    GetSettings: async () => settingsWith(false),
    UpdateSettings: async () => { throw new Error('database_error') },
  })
  try {
    setActivePinia(createPinia())
    const store = useTokenUsageVisibilityStore()

    assert.equal(await store.setShowTokenUsage(true), false)
    assert.equal(store.showTokenUsage, false, 'the setting must not change without a committed write')
  } finally {
    restore()
  }
})
