import { ref } from 'vue'

/**
 * One place that records a fatal, render-breaking error so the UI can show it
 * instead of a blank window.
 *
 * A packaged (non-dev) Wails window has no devtools and no console, so an
 * uncaught error is otherwise invisible: the screen just goes white with no
 * clue. Everything that can trap such an error — the app error handler, a
 * per-route ErrorBoundary, window.onerror and unhandledrejection — funnels
 * here, and FatalErrorOverlay renders the result.
 */
export interface FatalError {
  /** Where it was trapped: a Vue lifecycle hook, window.onerror, a rejection. */
  source: string
  message: string
  /** Stack when available. Never contains screen content or LLM payloads. */
  detail: string
}

export const fatalError = ref<FatalError | null>(null)

export function reportFatalError(source: string, cause: unknown): void {
  // Local console only. This is never a remote report: crash reporting stays
  // opt-in, and screen content must never leave the device (see CLAUDE.md).
  console.error(`[daygo fatal] ${source}`, cause)

  // Keep the first error. It is the root cause; later ones are usually cascades
  // triggered while the tree is already torn down.
  if (fatalError.value !== null) return

  const error = cause instanceof Error ? cause : undefined
  fatalError.value = {
    source,
    message: error?.message ?? String(cause),
    detail: error?.stack ?? '',
  }
}

export function clearFatalError(): void {
  fatalError.value = null
}
