import { createI18n } from 'vue-i18n'

import { messages } from '@/locales'

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
} as const

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: DEFAULT_LOCALE,
  fallbackLocale: FALLBACK_LOCALE,
  missingWarn: import.meta.env.DEV,
  fallbackWarn: import.meta.env.DEV,
  messages,
  datetimeFormats,
})

/**
 * Apply a locale to the i18n instance and to <html>.
 *
 * `lang` is what assistive tech and text rendering read;
 * `data-dg-lang-script` drives the display-font swap in styles/fonts.css
 * (Instrument Serif has no CJK glyphs).
 *
 * Go through useAppearanceStore().setLanguage() rather than calling this
 * directly.
 */
export function setLocale(locale: AppLocale): void {
  i18n.global.locale.value = locale

  const root = document.documentElement
  root.setAttribute('lang', locale)
  root.setAttribute('data-dg-lang-script', scriptTagOf(locale))
}
