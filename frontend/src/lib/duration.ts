import { useI18n } from 'vue-i18n'

/**
 * Wall-clock duration formatting shared by every page. Minutes in, a
 * localized "N 小时 M 分钟" style string out; the strings live under
 * common.duration so all pages render durations identically.
 *
 * A composable rather than a plain function so the keys go through the typed
 * `t` of the calling component's i18n scope.
 */
export function useDurationFormat(): (minutes: number) => string {
  const { t } = useI18n()
  return (minutes: number): string => {
    const rounded = Math.max(0, Math.round(minutes))
    const hours = Math.floor(rounded / 60)
    const remainder = rounded % 60
    if (hours === 0) return t('common.duration.minutes', { count: rounded })
    if (remainder === 0) return t('common.duration.hours', { count: hours })
    return t('common.duration.hoursMinutes', { hours, minutes: remainder })
  }
}
