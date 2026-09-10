import type { LanguagePreference } from '@/i18n/locales'

/*
 * Hand-written subset of the DTOs in docs/05-interface-contract.md §5.5.2.
 *
 * TEMPORARY. §5.5.5 rule 2 makes the generated api/generated/models.ts the one
 * source of DTO types; `wails generate module` cannot run yet because no
 * bindings exist. Field names below are deliberately identical to the Go JSON
 * tags so that swapping the import is a delete, not a rewrite.
 *
 * Where the Go side writes `string` but the contract comment lists a closed set,
 * the type here is narrowed to that set. That narrowing is what makes vue-tsc
 * reject an unhandled theme or protocol instead of letting it reach the UI.
 */

/** AppearanceSettingsDTO.Theme. Three-state; resolved to two before it reaches the DOM. */
export const APP_THEMES = ['system', 'light', 'dark'] as const

export type AppTheme = (typeof APP_THEMES)[number]

export interface AppearanceSettingsDTO {
  theme: AppTheme
  /** BCP 47. "" means follow the system — see i18n/locales.ts. */
  language: LanguagePreference
}

/** The wire protocol a custom endpoint speaks. */
export const PROVIDER_PROTOCOLS = ['openai', 'openai_responses', 'anthropic'] as const

export type ProviderProtocol = (typeof PROVIDER_PROTOCOLS)[number]

/**
 * One user-defined provider. `id` is an opaque generated identifier; the wire
 * protocol is its own field rather than something the id implies.
 *
 * `hasSecret` is the whole of what the UI may learn about the key: it is
 * write-only, and no field on this type ever carries the key itself.
 */
export interface ProviderDTO {
  id: string
  displayName: string
  protocol: ProviderProtocol
  endpoint: string
  model: string
  hasSecret: boolean
}

/**
 * ProviderRoutingDTO. `secondary` must be null when it would equal `primary` —
 * a routing that falls back to itself is a routing with no fallback.
 */
export interface ProviderRoutingDTO {
  primary: string
  secondary: string | null
}

/**
 * Draft for one connection probe. The secret crosses the Wails boundary for
 * this call only and is never persisted, logged or echoed back.
 */
export interface ProviderTestDraft {
  protocol: ProviderProtocol
  endpoint: string
  model: string
  secret: string
}

/** One probe outcome. A failed probe is a result, not an exception. */
export interface ProviderTestResult {
  ok: boolean
  model: string
  latencyMs: number
  capabilities: string[]
  errorCode: string
  message: string
}
