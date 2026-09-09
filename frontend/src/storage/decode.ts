/*
 * Decoders for values that come back from persistence.
 *
 * Anything read from disk is foreign input: it may have been written by an
 * older build, hand-edited, or truncated. §5.5.5 rule 4 bans `any` across a
 * boundary, so every read goes `unknown` -> validator -> typed value, and an
 * unusable value degrades to the caller's default instead of throwing.
 */

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/** Non-empty, trimmed string, or null. */
export function asText(value: unknown): string | null {
  if (typeof value !== 'string') return null
  const trimmed = value.trim()
  return trimmed === '' ? null : trimmed
}

/** String that is allowed to be empty (the DTO uses "" as a real value). */
export function asString(value: unknown): string | null {
  return typeof value === 'string' ? value : null
}

export function asBoolean(value: unknown): boolean | null {
  return typeof value === 'boolean' ? value : null
}

/** Narrow to one of a closed set of literals — the shape every enum-ish DTO field takes. */
export function asMember<T extends string>(
  value: unknown,
  members: readonly T[],
): T | null {
  return typeof value === 'string' && (members as readonly string[]).includes(value)
    ? (value as T)
    : null
}

export function asRecordArray(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value) ? value.filter(isRecord) : []
}
