import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import type { ProviderDTO, ProviderInput } from '../src/api/dto'
import { draftOf, emptyDraft, useProvidersStore, type ProviderDraft } from '../src/stores/providers'
import { USER_AGENT_PRESETS } from '../src/views/Settings/userAgentPresets'

/*
 * The per-provider User-Agent override is free text that ends up in an outgoing
 * HTTP header, so the store must reject a value with CR/LF or other control
 * bytes before any binding call, and must round-trip a legal one untouched.
 */

function providerFixture(overrides: Partial<ProviderDTO> = {}): ProviderDTO {
  return {
    id: 'p1',
    displayName: 'Fixture',
    protocol: 'openai',
    endpoint: 'https://example.invalid/v1',
    models: ['m'],
    maxImages: 0,
    userAgent: '',
    hasSecret: false,
    ...overrides,
  }
}

function stubWindow(backend: Record<string, unknown>): () => void {
  const previous = globalThis.window
  globalThis.window = { go: { app: { Backend: backend } } } as unknown as typeof globalThis.window
  return () => { globalThis.window = previous }
}

function draftWith(userAgent: string): ProviderDraft {
  return { ...emptyDraft(), displayName: 'Fixture', models: ['m'], userAgent }
}

test('a draft defaults the User-Agent to empty and reads a saved one back', () => {
  assert.equal(emptyDraft().userAgent, '')
  assert.equal(draftOf(providerFixture({ userAgent: 'Daygo/1.0' })).userAgent, 'Daygo/1.0')
})

test('saving sends the trimmed User-Agent and the list reads it back', async () => {
  const stored: ProviderDTO[] = []
  let saved: ProviderInput | null = null
  const restore = stubWindow({
    ListProviders: async () => stored.map((provider) => ({ ...provider })),
    GetProviderRouting: async () => ({ chain: [] }),
    SetProviderRouting: async () => {},
    AddProvider: async (input: ProviderInput) => {
      saved = input
      stored.push(providerFixture({ id: 'new', userAgent: input.userAgent }))
      return 'new'
    },
  })
  try {
    setActivePinia(createPinia())
    const store = useProvidersStore()

    assert.equal(await store.add(draftWith('  Daygo/1.0  ')), null)
    assert.equal(saved?.userAgent, 'Daygo/1.0', 'the value is trimmed before it crosses the boundary')
    assert.equal(store.providers[0]?.userAgent, 'Daygo/1.0')
  } finally {
    restore()
  }
})

test('an illegal User-Agent is rejected before any binding call', async () => {
  let calls = 0
  const restore = stubWindow({
    ListProviders: async () => [],
    GetProviderRouting: async () => ({ chain: [] }),
    SetProviderRouting: async () => {},
    AddProvider: async () => { calls++; return 'x' },
  })
  try {
    setActivePinia(createPinia())
    const store = useProvidersStore()

    const illegal: [string, string][] = [
      ['crlf', 'Daygo/1.0\r\nX-Injected: 1'],
      ['control byte', 'Daygo\u00001.0'],
      ['non-ascii', 'Daygo/1.0 中文'],
      ['too long', 'a'.repeat(513)],
    ]
    for (const [name, value] of illegal) {
      const errors = await store.add(draftWith(value))
      assert.notEqual(errors?.userAgent, undefined, `${name} must be rejected`)
    }
    assert.equal(calls, 0, 'no request may leave for an illegal header value')

    // The empty override and the maximum length are both legal.
    assert.equal(await store.add(draftWith('')), null)
    assert.equal(await store.add(draftWith('a'.repeat(512))), null)
  } finally {
    restore()
  }
})

test('every shipped preset is a legal header value', () => {
  assert.ok(USER_AGENT_PRESETS.length > 0)
  const keys = new Set<string>()
  for (const preset of USER_AGENT_PRESETS) {
    assert.ok(!keys.has(preset.key), `duplicate preset key ${preset.key}`)
    keys.add(preset.key)
    assert.ok(preset.value.length > 0 && preset.value.length <= 512, `${preset.key} length`)
    for (const ch of preset.value) {
      const code = ch.charCodeAt(0)
      assert.ok(code >= 0x20 && code <= 0x7e, `${preset.key} carries a non-printable byte`)
    }
  }
})
