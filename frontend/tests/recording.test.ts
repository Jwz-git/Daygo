import assert from 'node:assert/strict'
import test from 'node:test'

import { lifecycleOf } from '../src/stores/recording'

test('recording lifecycle accepts only contract states', () => {
  assert.equal(lifecycleOf('idle'), 'idle')
  assert.equal(lifecycleOf('starting'), 'starting')
  assert.equal(lifecycleOf('capturing'), 'capturing')
  assert.equal(lifecycleOf('paused'), 'paused')
  assert.equal(lifecycleOf('succeeded'), null)
  assert.equal(lifecycleOf(undefined), null)
})
