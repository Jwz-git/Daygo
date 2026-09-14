import { safeTimeZone } from './timeZone'

export function formatClockTime(
  unixSeconds: number,
  locale: string,
  timeZone?: string | null,
): string {
  return new Intl.DateTimeFormat(locale, {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: safeTimeZone(timeZone),
  }).format(new Date(unixSeconds * 1000))
}

export function formatTimeZoneName(
  timeZone: string | null | undefined,
  locale: string,
): string {
  const validTimeZone = safeTimeZone(timeZone)
  if (!validTimeZone) return ''

  const part = new Intl.DateTimeFormat(locale, {
    timeZone: validTimeZone,
    timeZoneName: 'long',
  })
    .formatToParts(new Date())
    .find((entry) => entry.type === 'timeZoneName')

  return part?.value ?? validTimeZone
}
