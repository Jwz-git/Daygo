import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { APP_THEMES, type AppTheme, type AppearanceSettingsDTO } from '@/api/dto'
import { setLocale } from '@/i18n'
import {
  DEFAULT_LANGUAGE,
  SUPPORTED_LOCALES,
  SYSTEM_LANGUAGE,
  normalizeLanguagePreference,
  resolveLanguage,
  type AppLocale,
  type LanguagePreference,
} from '@/i18n/locales'
import { asMember } from '@/storage/decode'
import { LEGACY_LANGUAGE_KEY, STORAGE_KEYS } from '@/storage/keys'
import {
  readLegacyString,
  readRecord,
  removeKey,
  writeRecord,
} from '@/storage/local'
import {
  DEFAULT_THEME,
  applyAppearance,
  resolveAppearance,
  watchSystemAppearance,
  type ResolvedAppearance,
} from '@/theme'

/** Every value the language <select> offers, "follow the system" first. */
export const LANGUAGE_PREFERENCES: readonly LanguagePreference[] = [
  SYSTEM_LANGUAGE,
  ...SUPPORTED_LOCALES,
]

function decodeStored(): AppearanceSettingsDTO {
  const raw = readRecord(STORAGE_KEYS.appearance)

  if (raw === null) {
    // Pre-envelope scaffold wrote the language alone, as a bare string.
    // Pick it up once so an existing choice survives, then drop the key.
    const legacy = normalizeLanguagePreference(
      readLegacyString(LEGACY_LANGUAGE_KEY),
    )
    removeKey(LEGACY_LANGUAGE_KEY)
    return { theme: DEFAULT_THEME, language: legacy ?? DEFAULT_LANGUAGE }
  }

  return {
    theme: asMember(raw.theme, APP_THEMES) ?? DEFAULT_THEME,
    language: normalizeLanguagePreference(raw.language) ?? DEFAULT_LANGUAGE,
  }
}

/**
 * Appearance settings: the interface theme and the interface language.
 *
 * The pair mirrors AppearanceSettingsDTO (docs/05-interface-contract.md
 * §5.5.2) because that is what will serve it: one store, one record, one
 * future UpdateSettings patch. Splitting theme and language into separate
 * stores would give one DTO two owners.
 *
 * `hydrate` and the setters are async even though today's persistence is
 * synchronous: when they become GetSettings() / UpdateSettings({ theme }) over
 * Wails, only these bodies change and no component is rewritten.
 */
export const useAppearanceStore = defineStore('appearance', () => {
  const theme = ref<AppTheme>(DEFAULT_THEME)
  const language = ref<LanguagePreference>(DEFAULT_LANGUAGE)

  /** What the DOM actually shows; "system" is already resolved away. */
  const appearance = ref<ResolvedAppearance>('light')

  /** What the UI is actually rendered in; "" is already resolved away. */
  const locale = computed<AppLocale>(() => resolveLanguage(language.value))

  const themes = APP_THEMES
  const languages = LANGUAGE_PREFERENCES

  /**
   * Live only while `theme` is "system". Held here rather than left attached
   * for the app's lifetime so the subscription has one visible owner and one
   * exit condition.
   */
  let stopSystemWatch: (() => void) | null = null

  function applyTheme(): void {
    const resolved = resolveAppearance(theme.value)
    appearance.value = resolved
    applyAppearance(resolved)
  }

  function syncSystemWatch(): void {
    const wanted = theme.value === 'system'
    if (wanted && stopSystemWatch === null) {
      stopSystemWatch = watchSystemAppearance(() => applyTheme())
    } else if (!wanted && stopSystemWatch !== null) {
      stopSystemWatch()
      stopSystemWatch = null
    }
  }

  function persist(): void {
    const dto: AppearanceSettingsDTO = {
      theme: theme.value,
      language: language.value,
    }
    writeRecord(STORAGE_KEYS.appearance, { ...dto })
  }

  async function hydrate(): Promise<void> {
    const stored = decodeStored()
    theme.value = stored.theme
    language.value = stored.language

    applyTheme()
    syncSystemWatch()
    setLocale(locale.value)
  }

  async function setTheme(next: AppTheme): Promise<void> {
    if (next === theme.value) return

    theme.value = next
    applyTheme()
    syncSystemWatch()
    persist()
  }

  async function setLanguage(next: LanguagePreference): Promise<void> {
    if (next === language.value) return

    language.value = next
    setLocale(locale.value)
    persist()
  }

  return {
    theme,
    language,
    appearance,
    locale,
    themes,
    languages,
    hydrate,
    setTheme,
    setLanguage,
  }
})
