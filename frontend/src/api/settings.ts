import { GetSettings, UpdateSettings } from '../../wailsjs/go/app/Backend'

import type { SettingsDTO, SettingsPatch } from '@/api/dto'

/** Thrown when the page runs in a plain browser, outside the Wails WebView. */
export const WAILS_UNAVAILABLE = 'wails_unavailable'

function requireBridge(): void {
  const bridge = (window as { go?: unknown }).go
  if (bridge === undefined) {
    throw new Error(WAILS_UNAVAILABLE)
  }
}

/** The effective settings, with backend defaults already applied. */
export async function getSettings(): Promise<SettingsDTO> {
  requireBridge()
  return GetSettings()
}

/**
 * Applies a partial patch. The return value is the post-normalization state,
 * which is the only authority to display — never the value that was sent.
 */
export async function updateSettings(patch: SettingsPatch): Promise<SettingsDTO> {
  requireBridge()
  return UpdateSettings(patch)
}
