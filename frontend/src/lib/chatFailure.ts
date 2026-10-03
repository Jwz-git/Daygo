/**
 * Resolve a terminal chat turn's error code to the line the user reads.
 *
 * The backend stores a closed machine code, never prose: the same row has to
 * read correctly in every language and stay correct after the user switches
 * language, so the wording lives in the bundles. `te` is load-bearing rather
 * than decorative — an unknown code (a row written by a newer build) must fall
 * back to the row's own text instead of rendering a raw `chat.failure.x` key.
 */
export function chatFailureMessage(
  message: { role: string; status: string; errorCode: string },
  te: (key: string) => boolean,
  t: (key: string) => string,
): string {
  if (message.role !== 'assistant') return ''
  if (message.status !== 'failed' && message.status !== 'canceled') return ''
  if (message.errorCode === '') return ''

  const key = `chat.failure.${message.errorCode}`
  return te(key) ? t(key) : ''
}
