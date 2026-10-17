import type { LanguagePreference } from '@/i18n/locales'

/**
 * Endonyms, deliberately not translated: someone looking for their language
 * has to recognise it in a UI they cannot currently read. Order follows
 * SUPPORTED_LOCALES so the list is not alphabetical in any one language.
 */
export const LANGUAGE_LABELS: Record<Exclude<LanguagePreference, ''>, string> = {
  'zh-CN': '简体中文',
  'zh-Hant': '繁體中文',
  en: 'English',
  ja: '日本語',
  ko: '한국어',
  de: 'Deutsch',
  fr: 'Français',
  es: 'Español',
  'pt-BR': 'Português (Brasil)',
}
