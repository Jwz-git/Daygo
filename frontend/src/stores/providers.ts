import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import {
  PROVIDER_PROTOCOLS,
  type ProviderDTO,
  type ProviderProtocol,
  type ProviderRoutingDTO,
} from '@/api/dto'
import { asMember, asRecordArray, asString, asText } from '@/storage/decode'
import { STORAGE_KEYS } from '@/storage/keys'
import { readRecord, writeRecord } from '@/storage/local'

/** Prefilled when a protocol is picked; the user is free to replace it. */
export const DEFAULT_ENDPOINTS: Record<ProviderProtocol, string> = {
  openai: 'https://api.openai.com/v1',
  openai_responses: 'https://api.openai.com/v1',
  anthropic: 'https://api.anthropic.com/v1',
}

export type ProviderField = 'displayName' | 'endpoint' | 'model'

export type ProviderFieldError = 'required' | 'invalidUrl'

export type ProviderErrors = Partial<Record<ProviderField, ProviderFieldError>>

/** What the form hands back. `secret` is never stored on a ProviderDTO. */
export interface ProviderDraft {
  displayName: string
  protocol: ProviderProtocol
  endpoint: string
  model: string
  secret: string
}

/** The persisted half of a provider — credential-free by construction. */
type StoredProvider = Omit<ProviderDTO, 'hasSecret'>

export function emptyDraft(): ProviderDraft {
  return {
    displayName: '',
    protocol: 'openai',
    endpoint: DEFAULT_ENDPOINTS.openai,
    model: '',
    secret: '',
  }
}

export function draftOf(provider: ProviderDTO): ProviderDraft {
  return {
    displayName: provider.displayName,
    protocol: provider.protocol,
    endpoint: provider.endpoint,
    model: provider.model,
    secret: '',
  }
}

/**
 * Accept only an absolute http(s) base URL, and keep just the part a request
 * path is appended to. A query or fragment on a base URL is always a mistake,
 * and anything outside http(s) — file:, data:, javascript: — must never reach
 * a fetch, so it is rejected here rather than filtered later.
 */
function normalizeEndpoint(raw: string): string | null {
  const trimmed = raw.trim()
  if (trimmed === '') return null

  let url: URL
  try {
    url = new URL(trimmed)
  } catch {
    return null
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return null

  url.search = ''
  url.hash = ''
  return url.toString().replace(/\/+$/, '')
}

function newProviderId(): string {
  const source = globalThis.crypto
  if (typeof source?.randomUUID === 'function') return source.randomUUID()

  // randomUUID needs a secure context. wails:// and http://localhost are one,
  // a plain-http preview is not. Ids only have to be unique, not unguessable.
  return `p-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function validate(
  draft: ProviderDraft,
): { ok: true; fields: Omit<StoredProvider, 'id'> } | { ok: false; errors: ProviderErrors } {
  const errors: ProviderErrors = {}

  const displayName = draft.displayName.trim()
  if (displayName === '') errors.displayName = 'required'

  const model = draft.model.trim()
  if (model === '') errors.model = 'required'

  const endpoint = normalizeEndpoint(draft.endpoint)
  if (draft.endpoint.trim() === '') {
    errors.endpoint = 'required'
  } else if (endpoint === null) {
    errors.endpoint = 'invalidUrl'
  }

  if (endpoint === null || Object.keys(errors).length > 0) {
    return { ok: false, errors }
  }
  return { ok: true, fields: { displayName, protocol: draft.protocol, endpoint, model } }
}

function decodeProvider(raw: Record<string, unknown>): StoredProvider | null {
  const id = asText(raw.id)
  const displayName = asText(raw.displayName)
  const protocol = asMember(raw.protocol, PROVIDER_PROTOCOLS)
  const endpoint = asText(raw.endpoint)
  const model = asText(raw.model)

  if (id === null || displayName === null || protocol === null) return null
  if (endpoint === null || model === null) return null
  if (normalizeEndpoint(endpoint) === null) return null

  return { id, displayName, protocol, endpoint, model }
}

function decodeRouting(
  raw: unknown,
  known: readonly StoredProvider[],
): ProviderRoutingDTO {
  const ids = new Set(known.map((provider) => provider.id))
  const empty: ProviderRoutingDTO = { primary: '', secondary: null }
  if (typeof raw !== 'object' || raw === null) return empty

  const record = raw as Record<string, unknown>
  const primary = asString(record.primary) ?? ''
  const secondary = asString(record.secondary)

  return normalizeRouting({
    primary: ids.has(primary) ? primary : '',
    secondary: secondary !== null && ids.has(secondary) ? secondary : null,
  })
}

/** Enforces the contract rule that `secondary` is null, never a copy of `primary`. */
function normalizeRouting(routing: ProviderRoutingDTO): ProviderRoutingDTO {
  const primary = routing.primary
  const secondary =
    routing.secondary === null || routing.secondary === primary || primary === ''
      ? null
      : routing.secondary
  return { primary, secondary }
}

/**
 * User-defined LLM providers and the routing between them.
 *
 * A provider is a name, one of two wire protocols, a base URL and a model id.
 * There is no built-in provider roster — see api/dto.ts ProviderDTO.
 *
 * SECRETS ARE NOT PERSISTED HERE.
 * §5.5.2 makes provider keys write-only and the system keychain their home
 * (service io.github.jwz-git.daygo.apikeys.<provider>), reached through
 * SetProviderSecret once that binding exists. Until then a key entered in the
 * UI is kept in memory for this session only (the connection-test binding
 * draws from it); writing it to localStorage would put a plaintext credential
 * in the WebView store. `hasSecret` is therefore derived from memory, never
 * read back from disk — so it cannot claim a key that is no longer there.
 */
export const useProvidersStore = defineStore('providers', () => {
  const entries = ref<StoredProvider[]>([])
  const routing = ref<ProviderRoutingDTO>({ primary: '', secondary: null })

  const hydrated = ref(false)

  /** Ids whose key was entered this session. Reactive half of `secrets`. */
  const secretIds = ref<string[]>([])
  /** id -> key. In-process only: never persisted, never logged, never rendered. */
  const secrets = new Map<string, string>()

  const providers = computed<ProviderDTO[]>(() =>
    entries.value.map((entry) => ({
      ...entry,
      hasSecret: secretIds.value.includes(entry.id),
    })),
  )

  const isEmpty = computed(() => entries.value.length === 0)

  function persist(): void {
    writeRecord(STORAGE_KEYS.providers, {
      providers: entries.value.map((entry) => ({ ...entry })),
      routing: { ...routing.value },
    })
  }

  function rememberSecret(id: string, secret: string): void {
    const trimmed = secret.trim()
    if (trimmed === '') return

    secrets.set(id, trimmed)
    if (!secretIds.value.includes(id)) secretIds.value = [...secretIds.value, id]
  }

  function forgetSecret(id: string): void {
    secrets.delete(id)
    secretIds.value = secretIds.value.filter((entry) => entry !== id)
  }

  async function hydrate(): Promise<void> {
    // The settings section mounts on every visit; the read happens once.
    if (hydrated.value) return
    hydrated.value = true

    const raw = readRecord(STORAGE_KEYS.providers)
    if (raw === null) return

    const decoded = asRecordArray(raw.providers)
      .map(decodeProvider)
      .filter((provider): provider is StoredProvider => provider !== null)

    entries.value = decoded
    routing.value = decodeRouting(raw.routing, decoded)
  }

  async function add(draft: ProviderDraft): Promise<ProviderErrors | null> {
    const result = validate(draft)
    if (!result.ok) return result.errors

    const id = newProviderId()
    entries.value = [...entries.value, { id, ...result.fields }]
    rememberSecret(id, draft.secret)

    // First provider has nothing to choose between: make it the primary.
    if (routing.value.primary === '') {
      routing.value = normalizeRouting({ ...routing.value, primary: id })
    }
    persist()
    return null
  }

  async function update(id: string, draft: ProviderDraft): Promise<ProviderErrors | null> {
    const result = validate(draft)
    if (!result.ok) return result.errors

    entries.value = entries.value.map((entry) =>
      entry.id === id ? { id, ...result.fields } : entry,
    )
    // An empty key field means "leave it alone", not "clear it" — clearing is
    // its own action, so a rename cannot silently drop the credential.
    rememberSecret(id, draft.secret)
    persist()
    return null
  }

  async function remove(id: string): Promise<void> {
    entries.value = entries.value.filter((entry) => entry.id !== id)
    forgetSecret(id)

    const next = { ...routing.value }
    if (next.primary === id) {
      // Promote the fallback rather than leaving the app with no primary.
      next.primary = next.secondary ?? entries.value[0]?.id ?? ''
    }
    if (next.secondary === id || next.secondary === next.primary) next.secondary = null

    routing.value = normalizeRouting(next)
    persist()
  }

  async function setPrimary(id: string): Promise<void> {
    routing.value = normalizeRouting({ ...routing.value, primary: id })
    persist()
  }

  async function setSecondary(id: string | null): Promise<void> {
    routing.value = normalizeRouting({ ...routing.value, secondary: id })
    persist()
  }

  async function clearSecret(id: string): Promise<void> {
    forgetSecret(id)
  }

  /** The in-memory key for a provider, if entered this session. Feeds the
   * connection-test binding; it is never rendered or persisted. */
  function secretOf(id: string): string | undefined {
    return secrets.get(id)
  }

  return {
    providers,
    routing,
    isEmpty,
    hydrate,
    add,
    update,
    remove,
    setPrimary,
    setSecondary,
    clearSecret,
    secretOf,
  }
})
