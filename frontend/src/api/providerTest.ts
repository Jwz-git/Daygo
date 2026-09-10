import { TestProviderConnection } from '../../wailsjs/go/app/Backend'

import type { ProviderTestDraft, ProviderTestResult } from '@/api/dto'

/** Thrown when the page runs in a plain browser, outside the Wails WebView. */
export const WAILS_UNAVAILABLE = 'wails_unavailable'

/**
 * One real probe against the draft provider. Callers get either the probe
 * result (including ok:false failures) or an Error whose message is either
 * WAILS_UNAVAILABLE or a `daygo:<code>: <message>` binding error.
 */
export async function testProviderConnection(
  draft: ProviderTestDraft,
): Promise<ProviderTestResult> {
  const bridge = (window as { go?: unknown }).go
  if (bridge === undefined) {
    throw new Error(WAILS_UNAVAILABLE)
  }
  return TestProviderConnection(draft)
}
