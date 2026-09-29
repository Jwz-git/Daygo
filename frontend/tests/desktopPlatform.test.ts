import assert from 'node:assert/strict'
import test from 'node:test'

import { parseDesktopPlatform } from '../src/lib/desktopPlatform'

test('parseDesktopPlatform accepts supported desktop platforms', () => {
  assert.equal(parseDesktopPlatform({ platform: 'windows' }), 'windows')
  assert.equal(parseDesktopPlatform({ platform: 'darwin' }), 'darwin')
  assert.equal(parseDesktopPlatform({ platform: 'linux' }), 'linux')
})

test('parseDesktopPlatform rejects malformed and unsupported values', () => {
  assert.equal(parseDesktopPlatform(null), 'unknown')
  assert.equal(parseDesktopPlatform({}), 'unknown')
  assert.equal(parseDesktopPlatform({ platform: 'freebsd' }), 'unknown')
})
