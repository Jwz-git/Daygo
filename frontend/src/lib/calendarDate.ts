const CALENDAR_DATE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/

/**
 * Shift an already-resolved calendar key without deriving a logical day.
 *
 * The backend remains the only owner of the 04:00 boundary and returns the
 * actual time window through GetDayContext. This helper only prepares the next
 * yyyy-MM-dd key that is sent back to that binding.
 */
export function shiftCalendarDate(value: string, dayOffset: number): string | null {
  const match = CALENDAR_DATE_PATTERN.exec(value)
  if (match === null) return null

  const year = Number(match[1])
  const month = Number(match[2]) - 1
  const day = Number(match[3])
  const date = new Date(Date.UTC(year, month, day))
  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month ||
    date.getUTCDate() !== day
  ) {
    return null
  }

  date.setUTCDate(date.getUTCDate() + dayOffset)
  const resultYear = date.getUTCFullYear()
  const resultMonth = String(date.getUTCMonth() + 1).padStart(2, '0')
  const resultDay = String(date.getUTCDate()).padStart(2, '0')
  return `${resultYear}-${resultMonth}-${resultDay}`
}

export function shiftWeekStart(value: string, weekOffset: number): string | null {
  return shiftCalendarDate(value, weekOffset * 7)
}

export function calendarDayQuery(value: unknown): string {
  return typeof value === 'string' && CALENDAR_DATE_PATTERN.test(value) ? value : ''
}
