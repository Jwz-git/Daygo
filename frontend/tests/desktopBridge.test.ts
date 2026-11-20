import assert from 'node:assert/strict'
import test from 'node:test'
import { isDesktopEnvironment, hasDesktopBridge, initializeDesktopBridge } from '../src/api/desktopBridge'

test('a runtime import and legacy globals do not establish a native v3 host', () => {
  const previous = globalThis.window
  try {
    for (const value of [{}, { go: {} }, { runtime: {} }, { _wails: {} }]) {
      globalThis.window = value as unknown as Window & typeof globalThis
      assert.equal(hasDesktopBridge(), false)
    }
    for (const OS of ['darwin', 'windows', 'linux']) {
      globalThis.window = { _wails: { environment: { OS } } } as unknown as Window & typeof globalThis
      assert.equal(hasDesktopBridge(), true)
    }
  } finally { globalThis.window = previous }
})

test('native environment accepts only the three desktop OS values', () => {
  for (const value of [null, undefined, {}, { platform: 'darwin' }, { OS: 'ios' }, { OS: 'browser' }]) {
    assert.equal(isDesktopEnvironment(value), false)
  }
})

test('native bootstrap waits for environment before the navigation marker exists', async () => {
  const previous = globalThis.window
  try {
    globalThis.window = { webkit: { messageHandlers: { external: { postMessage() {} } } } } as unknown as Window & typeof globalThis
    assert.equal(hasDesktopBridge(), false)
    await initializeDesktopBridge()
    assert.equal(hasDesktopBridge(), true)
    globalThis.window = {} as Window & typeof globalThis
    await initializeDesktopBridge()
    assert.equal(hasDesktopBridge(), false)
  } finally { globalThis.window = previous }
})
