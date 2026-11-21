import assert from 'node:assert/strict'
import test from 'node:test'
import { describeApplications, listInstalledApplications } from '@/api/application'
import { nativeFixtureWindow } from './nativeFixtureWindow'

test('concurrent application requests share pending list and icon lookups', async () => {
  const previous = globalThis.window
  const enumerations: string[] = []
  const resolution: string[][] = []
  const listed = { id: 'fixture-a', name: 'Anonymous A', iconDataUrl: '' }
  globalThis.window = nativeFixtureWindow({
    go: { app: { Backend: {
      ListInstalledApplications: async (language: string) => {
        enumerations.push(language)
        return [listed]
      },
      DescribeApplications: async (ids: string[]) => {
        resolution.push(ids)
        return ids.map((id) => ({ ...listed, id }))
      },
    } } },
  } as unknown as Window & typeof globalThis)
  try {
    assert.deepEqual(await Promise.all([
      listInstalledApplications('en'), listInstalledApplications('en'),
    ]), [[listed], [listed]])
    assert.deepEqual(enumerations, ['en'])

    const [first, second] = await Promise.all([
      describeApplications(['fixture-a', 'fixture-b']),
      describeApplications(['fixture-c', 'fixture-b', 'fixture-a']),
    ])
    assert.deepEqual(first.map((item) => item.id), ['fixture-a', 'fixture-b'])
    assert.deepEqual(second.map((item) => item.id), ['fixture-c', 'fixture-b', 'fixture-a'])
    assert.deepEqual(resolution, [['fixture-a', 'fixture-b'], ['fixture-c']])
  } finally {
    globalThis.window = previous
  }
})
