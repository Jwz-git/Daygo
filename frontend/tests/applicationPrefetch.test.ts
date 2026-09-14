import assert from 'node:assert/strict'
import test from 'node:test'

import { warmInstalledApplicationCache } from '../src/api/application'

test('application cache warming absorbs unavailable enumeration', async () => {
  const previousWindow = globalThis.window
  globalThis.window = {
    go: {
      app: {
        Backend: {
          ListInstalledApplications: async () => {
            throw new Error('daygo:native_unavailable: application enumeration failed')
          },
        },
      },
    },
  } as unknown as Window & typeof globalThis

  try {
    await assert.doesNotReject(warmInstalledApplicationCache('zh-CN'))
  } finally {
    globalThis.window = previousWindow
  }
})
