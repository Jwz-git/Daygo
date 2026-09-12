/**
 * A time zone the platform's Intl can actually format with.
 *
 * The backend sends an IANA id, but a malformed value — most infamously Go's
 * "Local" — makes `new Intl.DateTimeFormat(..., { timeZone })` throw a
 * RangeError that blanks the whole page. This degrades a bad value to
 * `undefined`, which tells Intl to use the runtime's own zone. On a desktop app
 * the window and the backend share one machine, so that fallback is also the
 * correct local zone.
 */
export function safeTimeZone(zone: string | null | undefined): string | undefined {
  if (typeof zone !== 'string' || zone === '') return undefined
  try {
    // Constructing is enough to validate; the format call sites reuse `zone`.
    new Intl.DateTimeFormat(undefined, { timeZone: zone })
    return zone
  } catch {
    return undefined
  }
}
