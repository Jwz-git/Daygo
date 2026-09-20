import {
  GetPermissionState,
  OpenSystemSettings,
  RequestScreenRecordingPermission,
} from '../../wailsjs/go/app/Backend'
import type { app } from '../../wailsjs/go/models'

export type PermissionState = app.PermissionDTO

/** The system-settings panes the backend accepts; mirrors platform.SettingsPane. */
export type SettingsPane = 'screen_recording' | 'notifications' | 'login_items'

/** Thrown when the page runs in a plain browser, outside the Wails WebView. */
export const WAILS_UNAVAILABLE = 'wails_unavailable'

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

/** Reads the observed screen-recording and notifications permission states. */
export async function getPermissionState(): Promise<PermissionState> {
  if (hasBridge()) return GetPermissionState()
  throw new Error(WAILS_UNAVAILABLE)
}

/*
 * Triggers the macOS screen-recording prompt on first use; a no-op once the
 * user has answered. The grant only takes effect after the app relaunches, so
 * the caller cannot observe a fresh grant in the same session.
 */
export async function requestScreenRecordingPermission(): Promise<void> {
  if (hasBridge()) return RequestScreenRecordingPermission()
  throw new Error(WAILS_UNAVAILABLE)
}

/** Opens one of the allowed system-settings panes. */
export async function openSystemSettings(pane: SettingsPane): Promise<void> {
  if (hasBridge()) return OpenSystemSettings(pane)
  throw new Error(WAILS_UNAVAILABLE)
}
