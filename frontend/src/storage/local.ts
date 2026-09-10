import { isRecord } from './decode'

/*
 * The only module in the app that touches localStorage.
 *
 * Why a module and not direct calls: the persistence backend is temporary. Once
 * the Wails bindings exist, GetSettings/UpdateSettings own these values (see
 * docs/05-interface-contract.md §5.5.2) and the stores switch to them.
 * Funnelling every read and write through here keeps that swap to the store
 * bodies plus this file, and keeps the key table auditable in one place.
 *
 * Nothing here may hold a credential. Provider API keys go to the Keychain via
 * SetProviderSecret once that binding exists — see stores/providers.ts.
 */

/**
 * Bumped only when a stored payload changes shape incompatibly. A record
 * written by a different version is ignored, so the store falls back to its
 * defaults rather than decoding fields that moved. When a real migration is
 * needed, branch on `v` here instead of widening every decoder.
 */
const SCHEMA_VERSION = 1

interface Envelope {
  v: number
  data: Record<string, unknown>
}

function isEnvelope(value: unknown): value is Envelope {
  return isRecord(value) && typeof value.v === 'number' && isRecord(value.data)
}

/**
 * Read one namespaced record. Returns null for: storage unavailable, key
 * absent, unparsable JSON, or a payload from another schema version.
 */
export function readRecord(key: string): Record<string, unknown> | null {
  let raw: string | null
  try {
    raw = localStorage.getItem(key)
  } catch {
    // Private mode, or site data blocked. Callers fall back to defaults.
    return null
  }
  if (raw === null) return null

  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return null
  }

  if (!isEnvelope(parsed) || parsed.v !== SCHEMA_VERSION) return null
  return parsed.data
}

export function writeRecord(key: string, data: Record<string, unknown>): void {
  const envelope: Envelope = { v: SCHEMA_VERSION, data }
  try {
    localStorage.setItem(key, JSON.stringify(envelope))
  } catch {
    // Quota or a locked-down store: the change still applies to this session.
  }
}

/** Read a bare (un-enveloped) string. Only used to pick up pre-envelope keys. */
export function readLegacyString(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

export function removeKey(key: string): void {
  try {
    localStorage.removeItem(key)
  } catch {
    // Nothing to do; a stale key is inert.
  }
}
