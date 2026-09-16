import assert from 'node:assert/strict'
import test from 'node:test'

import {
  builtInCategoryDetails,
  builtInCategoryKeys,
  categoryDetails,
  categoryLabel,
} from '../src/lib/categoryLabel'

test('maps every normalized built-in category to a stable translation key', () => {
  assert.deepEqual(Object.keys(builtInCategoryKeys), [
    'focus work',
    'communication',
    'learning',
    'research',
    'distraction',
    'personal',
  ])
})

test('localizes a built-in category regardless of casing and surrounding whitespace', () => {
  const translate = (key: string) => key === 'timeline.category.focusWork' ? '工作' : key

  assert.equal(categoryLabel('Focus Work', translate), '工作')
  assert.equal(categoryLabel('focus work', translate), '工作')
  assert.equal(categoryLabel('  FOCUS WORK  ', translate), '工作')
})

test('preserves user-defined category names', () => {
  assert.equal(categoryLabel('My Project', (key) => key), 'My Project')
})

test('localizes the shipped description of an untouched built-in category', () => {
  const translate = (key: string) =>
    key === 'timeline.categoryDetails.focusWork' ? '专注工作' : key

  assert.equal(
    categoryDetails('Focus Work', builtInCategoryDetails['focus work'] ?? '', translate),
    '专注工作',
  )
})

test('keeps a description the user rewrote on a built-in name', () => {
  assert.equal(categoryDetails('Focus Work', '我自己的说明', (key) => key), '我自己的说明')
})

test('stops localizing a built-in once the user renamed it', () => {
  const translate = (key: string) => `translated:${key}`

  assert.equal(categoryDetails('Deep Work', builtInCategoryDetails['focus work'] ?? '', translate), builtInCategoryDetails['focus work'])
  assert.equal(categoryDetails('Deep Work', 'notes', translate), 'notes')
})

test('preserves the description of a user-defined category', () => {
  assert.equal(categoryDetails('My Project', 'notes', (key) => key), 'notes')
})
