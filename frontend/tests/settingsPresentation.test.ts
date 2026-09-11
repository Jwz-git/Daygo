import assert from 'node:assert/strict'
import test from 'node:test'

import { formatByteSize, storageUsageRatio } from '../src/lib/settingsPresentation'

test('formatByteSize uses decimal disk units and clamps invalid input', () => {
  assert.equal(formatByteSize(1_500_000_000, 'en'), '1.5 GB')
  assert.equal(formatByteSize(-1, 'en'), '0 B')
  assert.equal(formatByteSize(Number.NaN, 'en'), '0 B')
})

test('storageUsageRatio distinguishes unlimited and clamps bounded usage', () => {
  assert.equal(storageUsageRatio(500, 1000), 0.5)
  assert.equal(storageUsageRatio(1500, 1000), 1)
  assert.equal(storageUsageRatio(500, 0), null)
})
