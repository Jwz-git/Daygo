/*
 * Every persisted key, in one table.
 *
 * Naming: `daygo.<settings domain>` — the domain matches the SettingsDTO group
 * it will be served from once the Wails bindings land
 * (docs/05-interface-contract.md §5.5.2), so a key never has to move when
 * ownership does.
 */
export const STORAGE_KEYS = {
  /** -> AppearanceSettingsDTO { theme, language } */
  appearance: 'daygo.appearance',
  /** -> ProviderRoutingDTO + the non-secret half of ProviderDTO. Never a secret. */
  providers: 'daygo.providers',
} as const

/**
 * Written by the first i18n scaffold as a bare string before the enveloped
 * record existed. Read once during hydration, then removed.
 */
export const LEGACY_LANGUAGE_KEY = 'daygo.appearance.language'
