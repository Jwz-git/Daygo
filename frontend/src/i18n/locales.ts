export const SUPPORTED_LOCALES = ['zh-CN', 'en'] as const

export type AppLocale = (typeof SUPPORTED_LOCALES)[number]

export const DEFAULT_LOCALE: AppLocale = 'zh-CN'
export const FALLBACK_LOCALE: AppLocale = 'en'

/** Writing system marker put on <html>, used by styles/fonts.css. */
export type LocaleScript = 'hans' | 'latn'

/**
 * AppearanceSettingsDTO.Language uses "" for "follow the system"
 * (docs/05-interface-contract.md §5.5.2), so the stored preference is a
 * locale *or* that sentinel — not merely a locale. Without it there is no way
 * to tell "the user picked Chinese" from "the system happens to be Chinese".
 */
export const SYSTEM_LANGUAGE = ''

export type LanguagePreference = typeof SYSTEM_LANGUAGE | AppLocale

/**
 * What a fresh install gets: Chinese, unconditionally.
 *
 * Deliberately NOT `SYSTEM_LANGUAGE`. Daygo's stated default language is
 * Chinese, and defaulting to the system probe would show English on an
 * English-language Mac — a different thing. Following the system is still
 * available, but as an explicit choice.
 */
export const DEFAULT_LANGUAGE: LanguagePreference = DEFAULT_LOCALE

function isSupported(tag: string): tag is AppLocale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(tag)
}

/**
 * Normalise a BCP 47 tag onto a locale that is actually shipped.
 *
 *   zh, zh-Hans, zh-CN, zh-SG  -> zh-CN
 *   zh-Hant, zh-TW, zh-HK      -> zh-CN  (see note below)
 *   en, en-US, en-GB, ...      -> en
 *   anything else              -> null
 *
 * Traditional Chinese falls back to Simplified rather than English: for a
 * Traditional reader Simplified is closer than English. Ship a zh-Hant bundle
 * and this single branch is what changes.
 */
export function normalizeLocale(tag: string | null | undefined): AppLocale | null {
  if (!tag) return null

  const cleaned = tag.trim()
  if (cleaned === '') return null
  if (isSupported(cleaned)) return cleaned

  const lower = cleaned.toLowerCase()
  if (lower === 'zh' || lower.startsWith('zh-') || lower.startsWith('zh_')) {
    return 'zh-CN'
  }
  if (lower === 'en' || lower.startsWith('en-') || lower.startsWith('en_')) {
    return 'en'
  }
  return null
}

/**
 * Narrow a stored or user-supplied value to a preference.
 *
 * Note the asymmetry with normalizeLocale: only the exact empty string is the
 * system sentinel. An unrecognised tag returns null so the caller can fall back
 * to DEFAULT_LANGUAGE rather than silently becoming "follow the system".
 */
export function normalizeLanguagePreference(
  value: unknown,
): LanguagePreference | null {
  if (value === SYSTEM_LANGUAGE) return SYSTEM_LANGUAGE
  if (typeof value !== 'string') return null
  return normalizeLocale(value)
}

/** The best shipped locale for the host's language list. */
export function systemLocale(): AppLocale {
  const candidates =
    typeof navigator === 'undefined'
      ? []
      : [...(navigator.languages ?? []), navigator.language]

  for (const candidate of candidates) {
    const resolved = normalizeLocale(candidate)
    if (resolved) return resolved
  }
  return DEFAULT_LOCALE
}

/** Preference -> the locale actually rendered. */
export function resolveLanguage(preference: LanguagePreference): AppLocale {
  return preference === SYSTEM_LANGUAGE ? systemLocale() : preference
}

export function scriptTagOf(locale: AppLocale): LocaleScript {
  return locale === 'zh-CN' ? 'hans' : 'latn'
}
