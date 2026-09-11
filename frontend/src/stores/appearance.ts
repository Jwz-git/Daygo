import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { APP_THEMES, type AppTheme, type AppearanceSettingsDTO } from '@/api/dto'
import {
  WAILS_UNAVAILABLE,
  getSettings,
  onSettingsChanged,
  updateSettings,
} from '@/api/settings'
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

interface StoredAppearance {
  value: AppearanceSettingsDTO
  shouldMigrate: boolean
  usedLegacyKey: boolean
}

function decodeStored(): StoredAppearance {
  const raw = readRecord(STORAGE_KEYS.appearance)
  if (raw !== null) {
    return {
      value: {
        theme: asMember(raw.theme, APP_THEMES) ?? DEFAULT_THEME,
        language: normalizeLanguagePreference(raw.language) ?? DEFAULT_LANGUAGE,
      },
      shouldMigrate: true,
      usedLegacyKey: false,
    }
  }

  const legacy = normalizeLanguagePreference(readLegacyString(LEGACY_LANGUAGE_KEY))
  return {
    value: {
      theme: DEFAULT_THEME,
      language: legacy ?? DEFAULT_LANGUAGE,
    },
    shouldMigrate: legacy !== null,
    usedLegacyKey: legacy !== null,
  }
}

function normalizeBackendAppearance(theme: string, language: string): AppearanceSettingsDTO {
  return {
    theme: asMember(theme, APP_THEMES) ?? DEFAULT_THEME,
    language: normalizeLanguagePreference(language) ?? DEFAULT_LANGUAGE,
  }
}

/**
 * Appearance remains immediately available in browser previews through the
 * versioned local record. Inside Wails, GetSettings is authoritative. An
 * existing local preference migrates once through UpdateSettings and is only
 * removed after the backend returns its normalized state, preventing dual
 * writes or preference loss when database startup fails.
 */
export const useAppearanceStore = defineStore('appearance', () => {
  const theme = ref<AppTheme>(DEFAULT_THEME)
  const language = ref<LanguagePreference>(DEFAULT_LANGUAGE)
  const appearance = ref<ResolvedAppearance>('light')
  const persistence = ref<'backend' | 'local' | 'unavailable'>('unavailable')
  const saving = ref(false)
  const locale = computed<AppLocale>(() => resolveLanguage(language.value))
  const themes = APP_THEMES
  const languages = LANGUAGE_PREFERENCES

  let stopSystemWatch: (() => void) | null = null
  let stopSettingsWatch: (() => void) | null = null

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

  function applyPreference(value: AppearanceSettingsDTO): void {
    theme.value = value.theme
    language.value = value.language
    applyTheme()
    syncSystemWatch()
    setLocale(locale.value)
  }

  function persistLocal(): void {
    writeRecord(STORAGE_KEYS.appearance, {
      theme: theme.value,
      language: language.value,
    })
  }

  async function reloadBackend(): Promise<void> {
    const settings = await getSettings()
    applyPreference(
      normalizeBackendAppearance(settings.appearance.theme, settings.appearance.language),
    )
  }

  function startSettingsWatch(): void {
    if (stopSettingsWatch !== null) return
    stopSettingsWatch = onSettingsChanged((keys) => {
      if (
        keys.includes('appearance.theme') ||
        keys.includes('appearance.language')
      ) {
        void reloadBackend()
      }
    })
  }

  async function hydrate(): Promise<void> {
    const stored = decodeStored()
    try {
      let settings = await getSettings()
      if (stored.shouldMigrate) {
        settings = await updateSettings({
          theme: stored.value.theme,
          language: stored.value.language,
        })
        removeKey(STORAGE_KEYS.appearance)
        if (stored.usedLegacyKey) removeKey(LEGACY_LANGUAGE_KEY)
      }
      persistence.value = 'backend'
      applyPreference(
        normalizeBackendAppearance(settings.appearance.theme, settings.appearance.language),
      )
      startSettingsWatch()
    } catch (error) {
      applyPreference(stored.value)
      if (error instanceof Error && error.message === WAILS_UNAVAILABLE) {
        persistence.value = 'local'
        persistLocal()
        if (stored.usedLegacyKey) removeKey(LEGACY_LANGUAGE_KEY)
      } else {
        persistence.value = 'unavailable'
      }
    }
  }

  async function setTheme(next: AppTheme): Promise<void> {
    if (next === theme.value || saving.value || persistence.value === 'unavailable') return
    if (persistence.value === 'local') {
      applyPreference({ theme: next, language: language.value })
      persistLocal()
      return
    }

    saving.value = true
    try {
      const settings = await updateSettings({ theme: next })
      applyPreference(
        normalizeBackendAppearance(settings.appearance.theme, settings.appearance.language),
      )
    } catch {
      await reloadBackend().catch(() => undefined)
    } finally {
      saving.value = false
    }
  }

  async function setLanguage(next: LanguagePreference): Promise<void> {
    if (next === language.value || saving.value || persistence.value === 'unavailable') return
    if (persistence.value === 'local') {
      applyPreference({ theme: theme.value, language: next })
      persistLocal()
      return
    }

    saving.value = true
    try {
      const settings = await updateSettings({ language: next })
      applyPreference(
        normalizeBackendAppearance(settings.appearance.theme, settings.appearance.language),
      )
    } catch {
      await reloadBackend().catch(() => undefined)
    } finally {
      saving.value = false
    }
  }

  return {
    theme,
    language,
    appearance,
    locale,
    persistence,
    saving,
    themes,
    languages,
    hydrate,
    setTheme,
    setLanguage,
  }
})
