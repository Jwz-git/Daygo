import assert from 'node:assert/strict'
import test from 'node:test'
import { effectScope, ref } from 'vue'

import { createInstalledApplicationPages } from '@/views/Settings/installedApplicationPages'
import type { ApplicationDTO } from '@/api/application'

test('privacy list pages bound mounted apps and icon reads while search spans the full list', async () => {
  const scope = effectScope()
  const queries: string[][] = []
  const fixture: ApplicationDTO[] = Array.from({ length: 120 }, (_, i) => ({
    id: `fixture-${i}`, name: `App ${i}`, iconDataUrl: '',
  }))
  const locale = ref('en')
  const pages = scope.run(() => createInstalledApplicationPages(locale, {
    list: async () => fixture,
    describe: async (ids) => {
      queries.push(ids)
      return ids.map((id) => ({ id, name: '', iconDataUrl: 'data:image/png;base64,fixture' }))
    },
  }))!
  const flush = () => new Promise<void>((done) => setImmediate(done))
  try {
    await pages.refreshInstalled()
    await flush()
    assert.equal(pages.total.value, 120)
    assert.equal(pages.shown.value.length, 48)
    assert.deepEqual(queries.flat(), fixture.slice(0, 48).map(({ id }) => id))
    assert.equal(pages.shown.value[0].iconDataUrl, 'data:image/png;base64,fixture')

    pages.navigate(1)
    await flush()
    assert.equal(pages.shown.value[0].id, 'fixture-48')
    assert.equal(queries.flat().length, 96)
    pages.query.value = 'fixture-119'
    await flush()
    assert.equal(pages.page.value, 0)
    assert.deepEqual(pages.shown.value.map(({ id }) => id), ['fixture-119'])
    assert.equal(queries.at(-1)?.[0], 'fixture-119')
  } finally {
    scope.stop()
  }
})

test('leaving privacy settings cancels the remaining icon batches', async () => {
  const scope = effectScope()
  const locale = ref('en')
  let calls = 0
  let complete: ((icons: ApplicationDTO[]) => void) | undefined
  let started: (() => void) | undefined
  const first = new Promise<void>((resolve) => { started = resolve })
  const pages = scope.run(() => createInstalledApplicationPages(locale, {
    list: async () => Array.from({ length: 48 }, (_, i) => ({ id: String(i), name: 'App', iconDataUrl: '' })),
    describe: () => {
      calls++
      started?.()
      return new Promise<ApplicationDTO[]>((resolve) => { complete = resolve })
    },
  }))!
  await pages.refreshInstalled()
  await first
  scope.stop()
  complete?.([])
  await new Promise<void>((done) => setImmediate(done))
  assert.equal(calls, 1)
})
