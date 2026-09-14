import assert from 'node:assert/strict'
import test from 'node:test'

import { builtInCategoryKeys, categoryLabel } from '../src/lib/categoryLabel'

test('maps every built-in category to a stable translation key', () => {
  assert.deepEqual(Object.keys(builtInCategoryKeys), [
    'Focus Work',
    'Communication',
    'Learning',
    'Research',
    'Distraction',
    'Personal',
  ])
})

test('localizes a built-in category', () => {
  assert.equal(categoryLabel('Focus Work', (key) => key === 'timeline.category.focusWork' ? '专注工作' : key), '专注工作')
})

test('preserves user-defined category names', () => {
  assert.equal(categoryLabel('My Project', (key) => key), 'My Project')
})
