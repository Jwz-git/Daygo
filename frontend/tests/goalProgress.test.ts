import assert from 'node:assert/strict'
import test from 'node:test'

import type { CategoryDTO } from '../src/api/dto'
import { compactHours, defaultGoalCategories, goalProgress } from '../src/views/Timeline/goalProgress'

const category = (id: string, name: string, isSystem = false): CategoryDTO => ({
  id, name, colorHex: '#123456', details: '', sortOrder: 0, isSystem, isIdle: false, createdAtTs: 0, updatedAtTs: 0,
})
const ref = (categoryId: string, name: string) => ({ categoryId, name, colorHex: '#abcdef', sortOrder: 0 })

const categories = [category('1', 'Work'), category('2', 'Distraction'), category('3', 'Learning'), category('9', 'System', true)]
const cards = [
  { category: 'Work', durationMinutes: 50 },
  { category: 'work', durationMinutes: 10 },
  { category: 'Learning', durationMinutes: 30 },
  { category: 'Distraction', durationMinutes: 25 },
  { category: 'System', durationMinutes: 40 },
  { category: 'Personal', durationMinutes: 15 },
]

// Hand-computed: focus = Work 60 + Learning 30, in the goal's order;
// distraction = 25; System and untracked categories never count.
test('goal progress sums card minutes per goal category', () => {
  const progress = goalProgress(
    { focusCategories: [ref('3', 'Learning'), ref('1', 'Work')], distractionCategories: [ref('2', 'Distraction')] },
    categories,
    cards,
  )
  assert.equal(progress.focusMinutes, 90)
  assert.deepEqual(progress.focusSegments.map((segment) => [segment.name, segment.minutes]), [['Learning', 30], ['Work', 60]])
  assert.equal(progress.distractionMinutes, 25)
})

// A renamed category is found through its id; a deleted one through the
// name saved with the goal. A system category in the goal still counts zero.
test('goal categories resolve by id first, then by the saved name', () => {
  const renamed = [category('1', 'Deep work'), category('9', 'System', true)]
  const progress = goalProgress(
    { focusCategories: [ref('1', 'Work'), ref('7', 'Learning'), ref('9', 'System')], distractionCategories: [] },
    renamed,
    [{ category: 'Deep work', durationMinutes: 20 }, { category: 'Learning', durationMinutes: 5 }, { category: 'System', durationMinutes: 9 }],
  )
  assert.deepEqual(progress.focusSegments.map((segment) => [segment.name, segment.minutes]), [['Deep work', 20], ['Learning', 5], ['System', 0]])
  assert.equal(progress.focusMinutes, 25)
  assert.equal(progress.distractionMinutes, 0)
})

test('compact hours drop a whole number decimal', () => {
  assert.equal(compactHours(120), '2')
  assert.equal(compactHours(270), '4.5')
  assert.equal(compactHours(0), '0')
  assert.equal(compactHours(100), '1.7')
})

// Dayflow defaultPlan: every user category but Distraction is focus, in sort
// order; system and idle categories are in neither set.
test('a new goal starts with every category but Distraction as focus', () => {
  const sorted = (id: string, name: string, sortOrder: number, flags: Partial<CategoryDTO> = {}): CategoryDTO =>
    ({ ...category(id, name), sortOrder, ...flags })
  const defaults = defaultGoalCategories([
    sorted('c', 'Learning', 3),
    sorted('a', 'Work', 1),
    sorted('d', ' distraction ', 2),
    sorted('e', 'Idle', 4, { isIdle: true }),
    sorted('s', 'System', 0, { isSystem: true }),
    sorted('b', 'Personal', 5),
  ])
  assert.deepEqual(defaults, { focus: ['a', 'c', 'b'], distraction: ['d'] })
})
