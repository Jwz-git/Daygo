import assert from 'node:assert/strict'
import test from 'node:test'
import { registerWindowView, registerWindowPlayer, restoreWindow, trackWindowAction,
  takeRestoredCardID, takeRestoredPlayer, windowSnapshot } from '../src/lib/windowResume'

test('only a ready timeline with a matching player supplies a bounded resume snapshot', () => {
  assert.equal(windowSnapshot(), null)
  let id = 7
  const page = registerWindowView(() => ({route: 'timeline', day: '2026-11-23', cardId: id}))
  const state = { progress: 31, rate: 60, expanded: true, playing: true, started: true }
  const player = registerWindowPlayer(() => ({cardId: 7, state}))
  const snapshot = windowSnapshot()
  assert.ok(snapshot)
  assert.deepEqual(snapshot.player, state)
  id = 8
  assert.equal(windowSnapshot(), null)
  restoreWindow(snapshot)
  assert.equal(takeRestoredCardID(), 7)
  assert.equal(takeRestoredCardID(), 0)
  assert.equal(takeRestoredPlayer(8), null)
  assert.deepEqual(takeRestoredPlayer(7), state)
  assert.equal(takeRestoredPlayer(7), null)
  player()
  assert.deepEqual(windowSnapshot(), { route: 'timeline', day: '2026-11-23', cardId: 8, player: null })
  const done = trackWindowAction()
  assert.equal(windowSnapshot(), null)
  done()
  assert.ok(windowSnapshot())
  page()
  assert.equal(windowSnapshot(), null)
})
