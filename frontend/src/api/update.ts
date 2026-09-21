import { CheckForUpdates, GetUpdaterState, SetAutomaticUpdateChecks } from '../../wailsjs/go/app/Backend'
import type { app } from '../../wailsjs/go/models'
import { WAILS_UNAVAILABLE } from './system'

export type UpdaterState = app.UpdaterStateDTO

function optionalString(value: unknown): string | undefined | false {
  return value === null || value === undefined ? undefined : typeof value === 'string' ? value : false
}

function optionalNumber(value: unknown): number | undefined | false {
  return value === null || value === undefined ? undefined : typeof value === 'number' ? value : false
}

export function parseUpdaterState(value: unknown): UpdaterState | null {
  if (typeof value !== 'object' || value === null) return null
  const row = value as Record<string, unknown>
  const availableVersion = optionalString(row.availableVersion)
  const lastCheckedAtTs = optionalNumber(row.lastCheckedAtTs)
  if (typeof row.automatic !== 'boolean' || typeof row.checking !== 'boolean') return null
  if (availableVersion === false || lastCheckedAtTs === false) return null
  return { automatic: row.automatic, checking: row.checking, availableVersion, lastCheckedAtTs }
}

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

/**
 * Reads the current update status. A build with no update adapter wired throws
 * native_unavailable through the binding, so callers hide the update surface
 * rather than showing a fabricated state.
 */
export async function getUpdaterState(): Promise<UpdaterState> {
  if (hasBridge()) {
    const state = parseUpdaterState(await GetUpdaterState())
    if (state !== null) return state
    throw new Error('invalid updater state')
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function setAutomaticUpdateChecks(enabled: boolean): Promise<void> {
  if (hasBridge()) return SetAutomaticUpdateChecks(enabled)
  throw new Error(WAILS_UNAVAILABLE)
}

/**
 * Starts an update check. interactive=true is a user-initiated check that also
 * reports "up to date"; interactive=false is the scheduled background check
 * that only surfaces when an update exists. Discovery arrives as the
 * update:available event, not this call's return value.
 */
export async function checkForUpdates(interactive: boolean): Promise<void> {
  if (hasBridge()) return CheckForUpdates(interactive)
  throw new Error(WAILS_UNAVAILABLE)
}

interface UpdateRuntime {
  EventsOnMultiple?: (name: string, callback: (...data: unknown[]) => void, maxCallbacks: number) => (() => void) | undefined
}

/**
 * Subscribes to the update:available state broadcast. The payload is the full
 * UpdaterState; the frontend replaces its local copy on each event (docs/05
 * §5.5.3) rather than treating the event as the source of truth. Returns an
 * unsubscribe function; a no-op outside a Wails host.
 */
export function onUpdateAvailable(callback: (state: UpdaterState) => void): () => void {
  const runtime = (window as { runtime?: UpdateRuntime }).runtime
  if (typeof runtime?.EventsOnMultiple !== 'function') return () => undefined
  return runtime.EventsOnMultiple('update:available', (...data: unknown[]) => {
    const value = data[0]
    const state = parseUpdaterState(value)
    if (state !== null) callback(state)
  }, -1) ?? (() => undefined)
}
