import assert from 'node:assert/strict'
import test from 'node:test'

import { LruCache } from '../src/lib/lruCache'

test('over-capacity insert evicts the least-recently-used entry', () => {
  const cache = new LruCache<string, number>(2)
  cache.set('a', 1)
  cache.set('b', 2)
  cache.set('c', 3) // evicts 'a', the oldest

  assert.equal(cache.has('a'), false)
  assert.equal(cache.get('b'), 2)
  assert.equal(cache.get('c'), 3)
  assert.equal(cache.size, 2)
})

test('reading a key refreshes its recency so it survives the next eviction', () => {
  const cache = new LruCache<string, number>(2)
  cache.set('a', 1)
  cache.set('b', 2)
  assert.equal(cache.get('a'), 1) // 'a' is now most-recent, 'b' oldest
  cache.set('c', 3) // evicts 'b', not 'a'

  assert.equal(cache.get('a'), 1)
  assert.equal(cache.has('b'), false)
  assert.equal(cache.get('c'), 3)
})

test('re-setting an existing key updates its value without growing size', () => {
  const cache = new LruCache<string, number>(2)
  cache.set('a', 1)
  cache.set('a', 9)

  assert.equal(cache.get('a'), 9)
  assert.equal(cache.size, 1)
})

test('a stored null is a present entry, distinct from an absent key', () => {
  // The favicon cache stores null to mark "resolved to nothing"; a miss must
  // stay undefined so it is re-resolved, while a null hit must not be.
  const cache = new LruCache<string, string | null>(4)
  cache.set('dead.example', null)

  assert.equal(cache.has('dead.example'), true)
  assert.equal(cache.get('dead.example'), null)
  assert.equal(cache.get('never.seen'), undefined)
})

test('byte budget preserves recency and accounts for replace/delete/clear', () => {
  const cache = new LruCache<string, string | null>(10, {
    maxWeight: 8,
    weigh: (value) => (value?.length ?? 0) * 2,
  })
  cache.set('a', '12')
  cache.set('b', '34')
  cache.get('a')
  cache.set('c', '5')
  assert.equal(cache.has('b'), false)
  assert.equal(cache.weight, 6)
  cache.set('a', '1')
  assert.equal(cache.weight, 4)
  cache.delete('c')
  assert.equal(cache.weight, 2)
  cache.set('a', 'oversized')
  assert.equal(cache.has('a'), false)
  assert.equal(cache.weight, 0)
  cache.set('empty', null)
  assert.equal(cache.get('empty'), null)
  cache.clear()
  assert.equal(cache.size, 0)
  assert.equal(cache.weight, 0)
})

test('entry and byte limits both hold under many different values', () => {
  const cache = new LruCache<string, string>(3, { maxWeight: 10, weigh: (s) => s.length })
  for (let index = 0; index < 1000; index += 1) {
    cache.set(String(index), 'x'.repeat(index % 8))
    assert.ok(cache.size <= 3)
    assert.ok(cache.weight <= 10)
  }
})
