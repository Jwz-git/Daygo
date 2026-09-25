import { createI18n } from 'vue-i18n'

import { initialMessages, loadLocale } from '@/locales'

import {
  DEFAULT_LOCALE,
  FALLBACK_LOCALE,
  scriptTagOf,
  type AppLocale,
} from './locales'

/*
 * Date and time boundary — see docs/05-interface-contract.md §5.3.2.
 *
 * The formats below are for display-only instants the frontend is free to
 * format itself (e.g. "last updated 14:32").
 *
 * These must NEVER be produced or reinterpreted here:
 *   - the logical day `day` (yyyy-MM-dd, 4 AM boundary) — only
 *     internal/timeutil produces it; ask the backend via GetDayContext("").
 *     Deriving it from `Date` in the frontend is off by a day between midnight
 *     and 4 AM.
 *   - the calendar day `standupDay` (midnight boundary).
 *   - the clock strings `start` / `end` on a timeline card — already localised
 *     by the backend; render them verbatim, never parse or re-format them.
 */
const datetimeFormats = {
  'zh-CN': {
    short: { hour: '2-digit', minute: '2-digit' },
    medium: { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' },
  },
  'zh-Hant': {
    short: { hour: '2-digit', minute: '2-digit' },
    medium: { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' },
  },
  en: {
    short: { hour: 'numeric', minute: '2-digit', hour12: true },
    medium: {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    },
  },
  ja: {
    short: { hour: 'numeric', minute: '2-digit' },
    medium: { month: 'long', day: 'numeric', hour: 'numeric', minute: '2-digit' },
  },
  ko: {
    short: { hour: 'numeric', minute: '2-digit', hour12: true },
    medium: {
      month: 'long',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    },
  },
  // de / fr / es / pt-BR are 24-hour by locale default, so hour12 is left
  // unset the same way it is for zh-CN rather than restating the default.
  // Intl orders the date parts itself, so day-before-month locales need no
  // reordering here.
  de: {
    short: { hour: '2-digit', minute: '2-digit' },
    medium: { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' },
  },
  fr: {
    short: { hour: '2-digit', minute: '2-digit' },
    medium: { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' },
  },
  es: {
    short: { hour: '2-digit', minute: '2-digit' },
    medium: { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' },
  },
  'pt-BR': {
    short: { hour: '2-digit', minute: '2-digit' },
    medium: { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' },
  },
} as const

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: DEFAULT_LOCALE,
  fallbackLocale: FALLBACK_LOCALE,
  missingWarn: Boolean(import.meta.env?.DEV),
  fallbackWarn: Boolean(import.meta.env?.DEV),
  messages: initialMessages,
  datetimeFormats,
})

/**
 * Make a locale active, loading its bundle first.
 *
 * Async because every locale but the default ships as its own chunk. Anything
 * that renders must await this before the first paint — bootstrap() in main.ts
 * resolves theme and language before mount for exactly this reason.
 *
 * Never rejects. A bundle that fails to load (corrupt install, an update that
 * removed the chunk) leaves the locale that is already active in place rather
 * than handing the renderer an empty message table. That is deliberately not
 * FALLBACK_LOCALE: at boot the active locale is the eager default, and later it
 * is the language the user is currently reading, which is a smaller surprise
 * than switching them to a third one.
 *
 * `lang` is what assistive tech and text rendering read;
 * `data-dg-lang-script` drives the display-font swap in styles/fonts.css
 * (Instrument Serif has no CJK glyphs).
 *
 * Go through useAppearanceStore().setLanguage() rather than calling this
 * directly.
 */
export async function setLocale(locale: AppLocale): Promise<void> {
  const bundle = await loadLocale(locale).catch((error: unknown) => {
    console.error(`daygo: could not load the ${locale} language bundle`, error)
    return null
  })
  if (bundle !== null) i18n.global.setLocaleMessage(locale, bundle)

  const active =
    bundle !== null ? locale : (i18n.global.locale.value as AppLocale)
  i18n.global.locale.value = active

  const root = document.documentElement
  root.setAttribute('lang', active)
  root.setAttribute('data-dg-lang-script', scriptTagOf(active))
}
