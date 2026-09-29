export const SUPPORTED_LOCALES = [
  'zh-CN',
  'zh-Hant',
  'en',
  'ja',
  'ko',
  'de',
  'fr',
  'es',
  'pt-BR',
] as const

export type AppLocale = (typeof SUPPORTED_LOCALES)[number]

export const DEFAULT_LOCALE: AppLocale = 'zh-CN'
export const FALLBACK_LOCALE: AppLocale = 'en'

/** Writing system marker put on <html>, used by styles/fonts.css. */
export type LocaleScript = 'hans' | 'hant' | 'jpan' | 'kore' | 'latn'

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

const SCRIPT_BY_LOCALE: Record<AppLocale, LocaleScript> = {
  'zh-CN': 'hans',
  'zh-Hant': 'hant',
  en: 'latn',
  ja: 'jpan',
  ko: 'kore',
  de: 'latn',
  fr: 'latn',
  es: 'latn',
  'pt-BR': 'latn',
}

function isSupported(tag: string): tag is AppLocale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(tag)
}

/** `zh-TW` -> `zh-tw`; underscores normalise so `zh_TW` folds identically. */
function canonicalize(tag: string): string {
  return tag.toLowerCase().replace(/_/g, '-')
}

/*
 * Script beats region: `zh-Hant-CN` is Traditional by the writer's own
 * declaration even though the region is the mainland, so the Hant branch is
 * tested before the region list. `zh-Hant` itself is the tag Daygo ships —
 * zh-TW / zh-HK / zh-MO are folded onto it rather than split into three
 * bundles, because the written form is what differs, not the vocabulary the
 * UI happens to use today.
 */
const TRADITIONAL_CHINESE_PREFIXES = ['zh-hant', 'zh-tw', 'zh-hk', 'zh-mo']

function startsWithLanguage(lowered: string, primary: string): boolean {
  return lowered === primary || lowered.startsWith(`${primary}-`)
}

/**
 * Normalise a BCP 47 tag onto a locale that is actually shipped.
 *
 *   zh, zh-Hans*, zh-CN, zh-SG    -> zh-CN
 *   zh-Hant*, zh-TW, zh-HK, zh-MO -> zh-Hant
 *   en, en-US, en-GB, ...         -> en
 *   ja, ja-JP, ...                -> ja
 *   ko, ko-KR, ...                -> ko
 *   de, de-AT, de-CH, ...         -> de
 *   fr, fr-CA, fr-BE, ...         -> fr
 *   es, es-ES, es-MX, es-419, ... -> es
 *   pt, pt-BR, pt-PT, ...         -> pt-BR
 *   anything else                 -> null
 *
 * A bare `zh` with no script or region is Simplified: it is the more common
 * reading, and Simplified readers parse Traditional text far worse than the
 * reverse, so the default has to favour them.
 *
 * The remaining languages fold every region onto one bundle. Only Portuguese
 * picks a region for its tag rather than a bare primary subtag, because the
 * Brazilian and European written standards diverge on ordinary UI vocabulary
 * (tela/ecrã, salvar/guardar) in a way German or French regional variants do
 * not; pt-BR is the larger audience and the software de-facto default.
 */
export function normalizeLocale(tag: string | null | undefined): AppLocale | null {
  if (!tag) return null

  const cleaned = tag.trim()
  if (cleaned === '') return null
  if (isSupported(cleaned)) return cleaned

  const lowered = canonicalize(cleaned)

  if (startsWithLanguage(lowered, 'zh')) {
    return TRADITIONAL_CHINESE_PREFIXES.some((prefix) => lowered.startsWith(prefix))
      ? 'zh-Hant'
      : 'zh-CN'
  }
  if (startsWithLanguage(lowered, 'ja')) return 'ja'
  if (startsWithLanguage(lowered, 'ko')) return 'ko'
  if (startsWithLanguage(lowered, 'en')) return 'en'
  if (startsWithLanguage(lowered, 'de')) return 'de'
  if (startsWithLanguage(lowered, 'fr')) return 'fr'
  if (startsWithLanguage(lowered, 'es')) return 'es'
  if (startsWithLanguage(lowered, 'pt')) return 'pt-BR'
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
  return SCRIPT_BY_LOCALE[locale]
}
