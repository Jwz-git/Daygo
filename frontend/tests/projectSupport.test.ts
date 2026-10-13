import assert from 'node:assert/strict'
import test from 'node:test'

import { openProjectSupport, PROJECT_SUPPORT_URL } from '../src/api/projectSupport'

test('project support opens the fixed repository in the Wails system browser', () => {
  const previous = globalThis.window
  const opened: string[] = []
  globalThis.window = { runtime: { BrowserOpenURL: (url: string) => opened.push(url) } } as unknown as Window & typeof globalThis
  try {
    assert.equal(openProjectSupport(), true)
    assert.deepEqual(opened, ['https://github.com/Jwz-git/Daygo'])
    assert.equal(PROJECT_SUPPORT_URL, opened[0])
  } finally {
    globalThis.window = previous
  }
})

test('project support leaves ordinary browser navigation to the link when Wails is absent', () => {
  const previous = globalThis.window
  try {
    for (const stub of [{}, { runtime: {} }, { runtime: null }]) {
      globalThis.window = stub as Window & typeof globalThis
      assert.equal(openProjectSupport(), false)
    }
  } finally {
    globalThis.window = previous
  }
})
