/**
 * Global message schema for vue-i18n.
 *
 * zh-CN is the type source: `$t()` keys are checked against it, so a typo or a
 * key that only exists in one bundle is a build error, not a runtime warning.
 */
import type { LocaleSchema } from '@/locales'

declare module 'vue-i18n' {
  export interface DefineLocaleMessage extends LocaleSchema {}
}

export {}
