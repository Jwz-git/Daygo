/**
 * Global message schema for vue-i18n.
 *
 * zh-CN is the type source: passing a bundle that is missing a key zh-CN has
 * (or vice versa) is a build error via the Record<AppLocale, LocaleSchema>
 * map in src/locales/index.ts.
 *
 * What this does NOT give you: vue-i18n's `t()` signature accepts any string
 * (`<Key extends string>(key: Key | ResourceKeys | ...)`), so a typo in a
 * `t('...')` call is NOT a compile error. Key coverage is enforced by
 * frontend/tests/i18nKeys.test.ts, which cross-checks every statically
 * referenced key against the zh-CN bundle and runs in the unit gate.
 */
import type { LocaleSchema } from '@/locales'

declare module 'vue-i18n' {
  export interface DefineLocaleMessage extends LocaleSchema {}
}

export {}
