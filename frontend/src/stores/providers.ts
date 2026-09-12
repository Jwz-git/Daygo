import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import {
  type ProviderDTO,
  type ProviderProtocol,
  type ProviderRoutingDTO,
  PROVIDER_PROTOCOLS,
} from '@/api/dto'
import {
  addProvider,
  deleteProvider,
  deleteProviderSecret,
  getProviderRouting,
  listProviders,
  setProviderRouting,
  setProviderSecret,
  updateProvider,
} from '@/api/providers'
import { asMember, asRecordArray, asString, asText } from '@/storage/decode'
import { STORAGE_KEYS } from '@/storage/keys'
import { readRecord, removeKey } from '@/storage/local'

/** Prefilled when a protocol is picked; the user is free to replace it. */
export const DEFAULT_ENDPOINTS: Record<ProviderProtocol, string> = {
  openai: 'https://api.openai.com/v1',
  openai_responses: 'https://api.openai.com/v1',
  anthropic: 'https://api.anthropic.com/v1',
}

export type ProviderField = 'displayName' | 'endpoint' | 'model'

export type ProviderFieldError = 'required' | 'invalidUrl'

export type ProviderErrors = Partial<Record<ProviderField, ProviderFieldError>>

/** What the form hands back. `secret` goes to the keychain, never to a DTO. */
export interface ProviderDraft {
  displayName: string
  protocol: ProviderProtocol
  endpoint: string
  model: string
  secret: string
}

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

function validate(draft: ProviderDraft): { ok: true } | { ok: false; errors: ProviderErrors } {
  const errors: ProviderErrors = {}

  if (draft.displayName.trim() === '') errors.displayName = 'required'
  if (draft.model.trim() === '') errors.model = 'required'

  if (draft.endpoint.trim() === '') {
    errors.endpoint = 'required'
  } else if (normalizeEndpoint(draft.endpoint) === null) {
    errors.endpoint = 'invalidUrl'
  }

  if (Object.keys(errors).length > 0) return { ok: false, errors }
  return { ok: true }
}

/** One localStorage row from the pre-binding era. */
interface LegacyProvider {
  id: string
  displayName: string
  protocol: ProviderProtocol
  endpoint: string
  model: string
}

function decodeLegacyProvider(raw: Record<string, unknown>): LegacyProvider | null {
  const id = asText(raw.id)
  const displayName = asText(raw.displayName)
  const protocol = asMember(raw.protocol, PROVIDER_PROTOCOLS)
  const endpoint = asText(raw.endpoint)
  const model = asText(raw.model)

  if (id === null || displayName === null || protocol === null) return null
  if (endpoint === null || model === null) return null
  return { id, displayName, protocol, endpoint, model }
}

/**
 * User-defined LLM providers and the routing chain between them.
 *
 * Persistence is the Go side: the providers table and the keychain via the
 * provider CRUD bindings (docs/05 §5.5.1). This store is a thin cache: every
 * write goes through the bindings and the authoritative list is re-pulled —
 * no optimistic updates (project rule).
 *
 * SECRETS ARE NOT PERSISTED OR HELD HERE. A key entered in the form crosses
 * to Go with the add/update call and lands in the keychain; `hasSecret` comes
 * back from the backend, which derives it from the keychain on read.
 */
export const useProvidersStore = defineStore('providers', () => {
  const providers = ref<ProviderDTO[]>([])
  const routing = ref<ProviderRoutingDTO>({ chain: [] })

  const hydrated = ref(false)
  /** Set when hydrate ran but bindings are unavailable (plain browser). */
  const unavailable = ref(false)

  const isEmpty = computed(() => providers.value.length === 0)

  /**
   * One-time migration from the pre-binding localStorage record. Runs only
   * when the Go list is empty and the legacy record exists; the record is
   * removed once processed so the migration is idempotent. Keys were never
   * persisted (memory-only by design), so they are genuinely lost — the user
   * re-enters them; providers and routing survive.
   */
  async function migrateLegacyRecord(): Promise<void> {
    const raw = readRecord(STORAGE_KEYS.providers)
    if (raw === null) return

    const legacyProviders = asRecordArray(raw.providers)
      .map(decodeLegacyProvider)
      .filter((provider): provider is LegacyProvider => provider !== null)
    if (legacyProviders.length === 0) {
      removeKey(STORAGE_KEYS.providers)
      return
    }

    // Old shape {primary, secondary} → ordered chain.
    const record = typeof raw.routing === 'object' && raw.routing !== null
      ? raw.routing as Record<string, unknown>
      : {}
    const primary = asString(record.primary) ?? ''
    const secondary = asString(record.secondary)

    const idMap = new Map<string, string>()
    for (const legacy of legacyProviders) {
      const id = await addProvider({
        displayName: legacy.displayName,
        protocol: legacy.protocol,
        endpoint: legacy.endpoint,
        model: legacy.model,
        secret: '',
      })
      idMap.set(legacy.id, id)
    }

    const chain: string[] = []
    for (const legacyId of [primary, secondary]) {
      if (legacyId === null || legacyId === '') continue
      const mapped = idMap.get(legacyId)
      if (mapped !== undefined && !chain.includes(mapped)) chain.push(mapped)
    }
    if (chain.length > 0) await setProviderRouting({ chain })

    removeKey(STORAGE_KEYS.providers)
  }

  async function refresh(): Promise<void> {
    const [list, chain] = await Promise.all([listProviders(), getProviderRouting()])
    providers.value = list
    routing.value = chain
  }

  async function hydrate(): Promise<void> {
    // The settings section mounts on every visit; the read happens once.
    if (hydrated.value) return
    hydrated.value = true

    try {
      const existing = await listProviders()
      if (existing.length === 0) await migrateLegacyRecord()
      await refresh()
    } catch {
      // Plain browser without the dev fixtures: the section shows its
      // unavailable state rather than pretending there are no providers.
      unavailable.value = true
    }
  }

  async function add(draft: ProviderDraft): Promise<ProviderErrors | null> {
    const result = validate(draft)
    if (!result.ok) return result.errors

    const id = await addProvider({
      displayName: draft.displayName.trim(),
      protocol: draft.protocol,
      endpoint: normalizeEndpoint(draft.endpoint) ?? draft.endpoint.trim(),
      model: draft.model.trim(),
      secret: draft.secret.trim(),
    })
    await refresh()

    // First provider has nothing to choose between: make it the primary.
    if (routing.value.chain.length === 0) {
      await setProviderRouting({ chain: [id] })
      await refresh()
    }
    return null
  }

  async function update(id: string, draft: ProviderDraft): Promise<ProviderErrors | null> {
    const result = validate(draft)
    if (!result.ok) return result.errors

    // An empty key field means "leave it alone", not "clear it" — clearing is
    // its own action, so a rename cannot silently drop the credential.
    await updateProvider(id, {
      displayName: draft.displayName.trim(),
      protocol: draft.protocol,
      endpoint: normalizeEndpoint(draft.endpoint) ?? draft.endpoint.trim(),
      model: draft.model.trim(),
      secret: draft.secret.trim(),
    })
    await refresh()
    return null
  }

  async function remove(id: string): Promise<void> {
    // DeleteProvider prunes routing and the keychain entry server-side.
    await deleteProvider(id)
    await refresh()
  }

  async function setChain(chain: string[]): Promise<void> {
    await setProviderRouting({ chain })
    await refresh()
  }

  async function clearSecret(id: string): Promise<void> {
    await deleteProviderSecret(id)
    await refresh()
  }

  /** Save a key without touching other fields (the secret-entry row). */
  async function saveSecret(id: string, secret: string): Promise<void> {
    if (secret.trim() === '') return
    await setProviderSecret(id, secret.trim())
    await refresh()
  }

  return {
    providers,
    routing,
    isEmpty,
    unavailable,
    hydrate,
    add,
    update,
    remove,
    setChain,
    clearSecret,
    saveSecret,
  }
})
