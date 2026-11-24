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

test('a hidden window retries the same request when busy state clears and cancels it on reopen', async () => {
  const { nextTick, ref } = await import('vue')
  const { onWindowRelease } = await import('../src/api/windowResume')
  let receive: ((token: number) => void) | undefined
  let notifyDOM: (() => void) | undefined
  const confirmations: number[] = []
  const visible = ref(false)
  let editorOpen = true
  const originals = Object.getOwnPropertyDescriptors(globalThis)
  const page = registerWindowView(() => ({ route: 'daily', day: '', cardId: 0 }))
  let unsubscribe = () => {}
  let disconnects = 0
  try {
    Object.assign(globalThis, {
      window: { _wails: { environment: { OS: 'darwin' } },
        runtime: { EventsOnMultiple: (_name: string, cb: (token: number) => void) => { receive = cb; return () => {} } },
        go: { app: { Backend: { ConfirmWindowRelease: (token: number) => { confirmations.push(token) } } } },
      },
      document: { body: {} },
      MutationObserver: class {
        constructor(callback: () => void) { notifyDOM = callback }
        observe() {}
        disconnect() { disconnects++ }
      },
    })
    unsubscribe = onWindowRelease(() => visible.value, () => !editorOpen)
    receive!(1)
    await nextTick()
    assert.deepEqual(confirmations, [])
    const finishSave = trackWindowAction()
    editorOpen = false
    notifyDOM?.()
    await nextTick()
    assert.deepEqual(confirmations, [])
    const finishSecondSave = trackWindowAction()
    finishSave()
    await nextTick()
    assert.deepEqual(confirmations, [])
    finishSecondSave()
    await nextTick()
    assert.deepEqual(confirmations, [1])

    editorOpen = true
    receive!(2)
    await nextTick()
    visible.value = true
    await nextTick()
    visible.value = false
    editorOpen = false
    notifyDOM?.()
    await nextTick()
    assert.deepEqual(confirmations, [1])
    receive!(3)
    await nextTick()
    assert.deepEqual(confirmations, [1, 3])
    assert.equal(disconnects, 3)
  } finally {
    unsubscribe()
    page()
    for (const key of ['window', 'document', 'MutationObserver'] as const) {
      if (originals[key]) Object.defineProperty(globalThis, key, originals[key])
      else Reflect.deleteProperty(globalThis, key)
    }
  }
})
