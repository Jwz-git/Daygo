import type { AppLocale } from '@/i18n/locales'

import zhCN from './zh-CN'

/**
 * zh-CN is the single source of truth for the message shape. Every bundle is
 * typed against it, so vue-tsc fails when one is missing a key rather than
 * letting it fall back silently at runtime.
 */
export type LocaleSchema = typeof zhCN

/*
 * Only the default locale sits in the initial chunk; every other bundle is its
 * own lazy chunk. Importing all nine eagerly put roughly half a megabyte of
 * copy in front of the first paint for a user who reads exactly one language.
 *
 * Typing the map as Record<AppLocale, () => Promise<{ default: LocaleSchema }>>
 * preserves the compile-time key check the eager map used to provide: adding a
 * locale to SUPPORTED_LOCALES without a loader, or shipping a bundle whose shape
 * has drifted from zh-CN, both fail vue-tsc.
 */
const LOCALE_LOADERS: Record<AppLocale, () => Promise<{ default: LocaleSchema }>> = {
  'zh-CN': async () => ({ default: zhCN }),
  'zh-Hant': () => import('./zh-Hant'),
  en: () => import('./en'),
  ja: () => import('./ja'),
  ko: () => import('./ko'),
  de: () => import('./de'),
  fr: () => import('./fr'),
  es: () => import('./es'),
  'pt-BR': () => import('./pt-BR'),
}

/**
 * Message tables that must exist before the first paint. Exactly the default
 * locale: the app boots in it, so waiting on a network-free dynamic import
 * before showing anything would be a self-inflicted flash of unstyled keys.
 */
export const initialMessages = { 'zh-CN': zhCN }

const bundles = new Map<AppLocale, LocaleSchema>(
  Object.entries(initialMessages) as [AppLocale, LocaleSchema][],
)

/** Whether this locale's bundle is already in memory. */
export function isLocaleLoaded(locale: AppLocale): boolean {
  return bundles.has(locale)
}

/**
 * Resolve a locale's bundle, loading its chunk on first use.
 *
 * Rejects if the chunk cannot be fetched — a corrupt install or an update that
 * removed it. Callers decide how to degrade; see setLocale in src/i18n/index.ts.
 */
export async function loadLocale(locale: AppLocale): Promise<LocaleSchema> {
  const cached = bundles.get(locale)
  if (cached) return cached

  const bundle = (await LOCALE_LOADERS[locale]()).default
  bundles.set(locale, bundle)
  return bundle
}
