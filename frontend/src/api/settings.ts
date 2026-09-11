import { GetSettings, UpdateSettings } from '../../wailsjs/go/app/Backend'

import {
  applyDevelopmentSettingsPatch,
  getSettingsDevelopmentFixture,
} from '@/api/developmentFixtures'
import type { SettingsDTO, SettingsPatch } from '@/api/dto'

/** Thrown when the page runs outside the Wails WebView and no fixture applies. */
export const WAILS_UNAVAILABLE = 'wails_unavailable'

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

/*
 * Dev-browser stand-in: vite serves frontend/dev-fixtures/settings.json at
 * /__daygo_dev__/settings. Patches apply to this in-memory copy only, so the
 * whole settings UI can be exercised in a plain browser. Never in production
 * (import.meta.env.DEV guard inside the fixture fetch) and never a substitute
 * for the backend's normalization.
 */
let developmentSettings: SettingsDTO | null = null

async function loadDevelopmentSettings(): Promise<SettingsDTO | null> {
  developmentSettings ??= await getSettingsDevelopmentFixture()
  return developmentSettings
}

/** The effective settings, with backend defaults already applied. */
export async function getSettings(): Promise<SettingsDTO> {
  if (hasBridge()) return (await GetSettings()) as unknown as SettingsDTO

  const dev = await loadDevelopmentSettings()
  if (dev !== null) return dev
  throw new Error(WAILS_UNAVAILABLE)
}

/**
 * Applies a partial patch. The return value is the post-normalization state,
 * which is the only authority to display — never the value that was sent.
 */
export async function updateSettings(patch: SettingsPatch): Promise<SettingsDTO> {
  if (hasBridge()) return (await UpdateSettings(patch)) as unknown as SettingsDTO

  const dev = await loadDevelopmentSettings()
  if (dev === null) throw new Error(WAILS_UNAVAILABLE)
  developmentSettings = applyDevelopmentSettingsPatch(dev, patch)
  return developmentSettings
}

interface SettingsChangedPayload {
  keys: string[]
}

interface WailsRuntime {
  EventsOnMultiple: (
    eventName: string,
    callback: (...data: unknown[]) => void,
    maxCallbacks: number,
  ) => () => void
}

function isWailsRuntime(value: unknown): value is WailsRuntime {
  return typeof value === 'object' && value !== null &&
    'EventsOnMultiple' in value && typeof value.EventsOnMultiple === 'function'
}

function settingsChangedPayload(value: unknown): SettingsChangedPayload | null {
  if (typeof value !== 'object' || value === null || !('keys' in value)) return null
  if (!Array.isArray(value.keys) || !value.keys.every((key) => typeof key === 'string')) return null
  return { keys: value.keys }
}

export function onSettingsChanged(callback: (keys: readonly string[]) => void): () => void {
  if (!('runtime' in window) || !isWailsRuntime(window.runtime)) return () => undefined
  return window.runtime.EventsOnMultiple('settings:changed', (raw: unknown) => {
    const payload = settingsChangedPayload(raw)
    if (payload !== null) callback(payload.keys)
  }, -1) ?? (() => undefined)
}
