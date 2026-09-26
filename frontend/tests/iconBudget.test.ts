import assert from 'node:assert/strict'
import test from 'node:test'
import { describeApplications } from '../src/api/application'
import { fetchFaviconDataUrl } from '../src/lib/favicon'

test('application budget evicts memo only and oversized results remain usable', async () => {
  const previousWindow = globalThis.window
  const calls: string[][] = []
  const icon = 'data:image/png;base64,' + 'a'.repeat(300_000)
  globalThis.window = { go: { app: { Backend: {
    DescribeApplications: async (ids: string[]) => {
      calls.push(ids)
      return ids.map((id) => ({ id, name: 'Anonymous', iconDataUrl: id === 'budget-huge' ? icon.repeat(15) : icon }))
    },
  } } } } as unknown as Window & typeof globalThis
  try {
    const ids = Array.from({ length: 40 }, (_, i) => `budget-app-${i}`)
    await describeApplications(ids)
    assert.equal((await describeApplications(['budget-app-39']))[0]?.iconDataUrl, icon)
    assert.equal(calls.length, 1)
    assert.equal((await describeApplications(['budget-app-0']))[0]?.iconDataUrl, icon)
    assert.equal(calls.length, 2)
    for (let i = 0; i < 2; i += 1) {
      assert.equal((await describeApplications(['budget-huge']))[0]?.iconDataUrl, icon.repeat(15))
    }
    assert.equal(calls.length, 4)
  } finally { globalThis.window = previousWindow }
})

test('favicon budget supports eviction, oversize bypass and concurrent de-duplication', async () => {
  const previousWindow = globalThis.window
  const previousFetch = globalThis.fetch
  const previousReader = globalThis.FileReader
  let calls = 0
  let result = 'data:image/png;base64,' + 'a'.repeat(200_000)
  globalThis.window = { setTimeout, clearTimeout } as unknown as Window & typeof globalThis
  globalThis.fetch = async () => {
    calls += 1
    return new Response(new Blob(['anonymous'], { type: 'image/png' }))
  }
  globalThis.FileReader = class {
    result: string | null = null
    onload: (() => void) | null = null
    onerror: (() => void) | null = null
    readAsDataURL(): void { this.result = result; this.onload?.() }
  } as unknown as typeof FileReader
  try {
    const first = fetchFaviconDataUrl('budget-icon0.example')
    const shared = fetchFaviconDataUrl('budget-icon0.example')
    assert.equal(first, shared)
    assert.equal(await first, result)
    for (let i = 1; i < 40; i += 1) await fetchFaviconDataUrl(`budget-icon${i}.example`)
    const count = calls
    assert.equal(await fetchFaviconDataUrl('budget-icon39.example'), result)
    assert.equal(calls, count)
    assert.equal(await fetchFaviconDataUrl('budget-icon0.example'), result)
    assert.equal(calls, count + 1)
    result = 'data:image/png;base64,' + 'a'.repeat(2_200_000)
    assert.equal(await fetchFaviconDataUrl('budget-huge.example'), result)
    assert.equal(await fetchFaviconDataUrl('budget-huge.example'), result)
    assert.equal(calls, count + 3)
  } finally {
    globalThis.window = previousWindow
    globalThis.fetch = previousFetch
    globalThis.FileReader = previousReader
  }
})
