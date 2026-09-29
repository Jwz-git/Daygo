import assert from 'node:assert/strict'
import test from 'node:test'
import { VisibilitySource } from '../src/lib/uiVisibility'
import { PlaybackScheduler } from '../src/lib/playbackScheduler'

test('late snapshot cannot overwrite a hide event, focus alone keeps visibility', async () => {
  let resolve!: (visible: boolean) => void
  let event!: (visible: boolean) => void
  let stopped = false
  const changes: boolean[] = []
  const source = new VisibilitySource(
    (callback) => { event = callback; return () => { stopped = true } },
    () => new Promise<boolean>((done) => { resolve = done }),
    (visible) => changes.push(visible),
  )
  const initial = source.start()
  event(false)
  resolve(true)
  await initial
  assert.deepEqual(changes, [false])
  source.setDocumentVisible(true)
  assert.equal(source.visible, false)
  event(true)
  assert.equal(source.visible, true)
  source.stop()
  event(false)
  assert.equal(source.visible, true)
  assert.equal(stopped, true)
})

test('document visibility combines with native visibility and newer snapshots win', async () => {
  const pending: ((value: boolean) => void)[] = []
  const source = new VisibilitySource(() => () => undefined,
    () => new Promise<boolean>((resolve) => pending.push(resolve)), () => undefined)
  const initial = source.start()
  const refreshed = source.refresh()
  pending[1]!(false)
  await refreshed
  pending[0]!(true)
  await initial
  assert.equal(source.visible, false)
  source.setDocumentVisible(false)
  const shown = source.refresh()
  pending[2]!(true)
  await shown
  assert.equal(source.visible, false)
  source.setDocumentVisible(true)
  assert.equal(source.visible, true)
  source.stop()
})

test('hidden playback freezes progress, resumes intention and cancels owned RAF', () => {
  let next = 0
  const callbacks = new Map<number, (now: number) => void>()
  let progress = 0
  const scheduler = new PlaybackScheduler((delta) => { progress += delta },
    (callback) => { callbacks.set(++next, callback); return next },
    (id) => { callbacks.delete(id) })
  const tick = (now: number): void => {
    const [id, callback] = callbacks.entries().next().value!
    callbacks.delete(id)
    callback(now)
  }
  scheduler.setPlaying(true)
  tick(1000); tick(2000)
  assert.equal(progress, 1)
  scheduler.setVisible(false)
  assert.equal(callbacks.size, 0)
  scheduler.setVisible(true)
  tick(100000); tick(101000)
  assert.equal(progress, 2)
  scheduler.setPlaying(false)
  scheduler.setVisible(false); scheduler.setVisible(true)
  assert.equal(callbacks.size, 0)
  scheduler.setPlaying(true); scheduler.dispose()
  assert.equal(callbacks.size, 0)
})
