import assert from 'node:assert/strict'
import test from 'node:test'

const builtInCategoryKeys: Record<string, string> = {
  'Focus Work': 'focusWork',
  Communication: 'communication',
  Learning: 'learning',
  Research: 'research',
  Distraction: 'distraction',
  Personal: 'personal',
}

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

test('does not map user-defined category names', () => {
  assert.equal(builtInCategoryKeys['My Project'], undefined)
})
