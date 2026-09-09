import type { AppLocale } from '@/i18n/locales'

import en from './en'
import zhCN from './zh-CN'

/**
 * zh-CN is the single source of truth for the message shape. Typing the map as
 * Record<AppLocale, LocaleSchema> makes vue-tsc fail when another bundle is
 * missing a key rather than letting it fall back silently at runtime.
 */
export type LocaleSchema = typeof zhCN

export const messages: Record<AppLocale, LocaleSchema> = {
  'zh-CN': zhCN,
  en,
}
