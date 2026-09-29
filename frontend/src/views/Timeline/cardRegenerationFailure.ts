// Wails errors cross the binding as daygo:<code>: <sanitized message>.
// Only the closed code is used for UI copy: the message may contain diagnostic
// details that should not be rendered in a personal timeline.
export function cardRegenerationFailureKey(cause: unknown): string {
  const message = typeof cause === 'string'
    ? cause
    : cause instanceof Error
      ? cause.message
      : typeof cause === 'object' && cause !== null && 'message' in cause &&
          typeof cause.message === 'string'
        ? cause.message
        : ''
  const code = /^daygo:([a-z_]+):/.exec(message)?.[1]
  switch (code) {
    case 'provider_not_configured':
    case 'provider_failed':
    case 'conflict':
    case 'invalid_argument':
    case 'not_capture_owner':
    case 'not_found':
    case 'canceled':
      return `timeline.reprocess.error.${code}`
    default:
      return 'timeline.reprocess.error.application'
  }
}
