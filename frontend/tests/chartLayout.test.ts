import assert from 'node:assert/strict'
import test from 'node:test'

import { squarify } from '../src/lib/chartLayout'

const bounds = { x: 10, y: 20, width: 600, height: 400 }

test('squarify keeps tiles inside the bounds with areas proportional to value', () => {
  const values = [6, 6, 4, 3, 2, 2, 1]
  const placements = squarify(values, (value) => value, bounds)
  assert.equal(placements.length, values.length)
  const total = values.reduce((sum, value) => sum + value, 0)
  for (const { item, rect } of placements) {
    assert.ok(rect.x >= bounds.x - 1e-6 && rect.y >= bounds.y - 1e-6)
    assert.ok(rect.x + rect.width <= bounds.x + bounds.width + 1e-6)
    assert.ok(rect.y + rect.height <= bounds.y + bounds.height + 1e-6)
    const expected = (item / total) * bounds.width * bounds.height
    assert.ok(Math.abs(rect.width * rect.height - expected) < 1e-6, 'area matches the share')
  }
  // No two tiles overlap.
  for (let a = 0; a < placements.length; a += 1) {
    for (let b = a + 1; b < placements.length; b += 1) {
      const p = placements[a].rect
      const q = placements[b].rect
      const overlapX = Math.min(p.x + p.width, q.x + q.width) - Math.max(p.x, q.x)
      const overlapY = Math.min(p.y + p.height, q.y + q.height) - Math.max(p.y, q.y)
      assert.ok(overlapX <= 1e-6 || overlapY <= 1e-6, `tiles ${a} and ${b} overlap`)
    }
  }
})

test('squarify drops non-positive values and handles an empty input', () => {
  assert.deepEqual(squarify([0, -2], (value) => value, bounds), [])
  assert.equal(squarify([3, 0], (value) => value, bounds).length, 1)
})

test('squarify gaps separate tiles but not the outer edge', () => {
  const placements = squarify([1, 1], (value) => value, { x: 0, y: 0, width: 200, height: 100 }, 4)
  const [left, right] = placements.map((placement) => placement.rect)
  assert.equal(left.x + left.width + 4, right.x, 'a 4px gap between the two tiles')
  assert.equal(right.x + right.width, 200, 'the last tile reaches the edge')
  assert.equal(left.height, 100)
})
