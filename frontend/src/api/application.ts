import {
  DescribeApplications,
  GetBlockedApplications,
  GetPrivacyCompatibility,
  ListInstalledApplications,
  PickApplication,
} from '../../wailsjs/go/app/Backend'

import { getApplicationNamesDevelopmentFixture } from '@/api/developmentFixtures'
import { WAILS_UNAVAILABLE, getSettings } from '@/api/settings'

/**
 * ApplicationDTO mirrors docs/05 §5.5.2: the display identity of one
 * application. `name` and `iconDataUrl` are display data resolved by the
 * native layer; the persisted privacy setting holds only `id`. A name that came
 * back empty means the platform could not resolve the bundle, and the caller
 * shows `id` instead of inventing a label.
 */
export interface ApplicationDTO {
  id: string
  name: string
  iconDataUrl: string
}

export interface PrivacyCompatibilityDTO {
  platform: string
  version: string
  build: number
  minimumBuild: number
  supported: boolean
}

export async function getPrivacyCompatibility(): Promise<PrivacyCompatibilityDTO | null> {
  if (!('go' in window) || window.go === undefined) return null
  return (await GetPrivacyCompatibility()) as unknown as PrivacyCompatibilityDTO
}

/**
 * Opens the platform's native application picker. Returns null when the user
 * cancels. Outside the Wails app there is no picker and no stand-in, so this
 * reports the unavailable state rather than opening a text prompt.
 */
export async function pickApplication(): Promise<ApplicationDTO | null> {
  if (!('go' in window) || window.go === undefined) throw new Error(WAILS_UNAVAILABLE)
  return (await PickApplication()) as unknown as ApplicationDTO | null
}

/**
 * The configured privacy list in configuration order, with whatever display
 * identity the platform can resolve. The backend reads the identifiers itself,
 * so the browser stand-in is the only path that needs the dev fixture.
 */
export async function getBlockedApplications(): Promise<ApplicationDTO[]> {
  if ('go' in window && window.go !== undefined) {
    return (await GetBlockedApplications()) as unknown as ApplicationDTO[]
  }

  const settings = await getSettings()
  const names = (await getApplicationNamesDevelopmentFixture()) ?? {}
  return settings.privacy.blockedApplicationIds.map((id) => ({
    id,
    name: names[id] ?? '',
    iconDataUrl: '',
  }))
}

/*
 * One-entry enumeration cache keyed by the requested language, plus a
 * persistent icon memo. The privacy grid enumerates on mount and re-enumerates
 * when the UI language changes; without the cache every visit pays the
 * filesystem walk and every icon batch again. The cache is filled by
 * prefetchInstalledApplications at shell startup, so opening the settings page
 * renders from memory.
 */
let installedCache: { language: string; apps: ApplicationDTO[] } | null = null
const identityCache = new Map<string, ApplicationDTO>()

export function prefetchInstalledApplications(language: string): void {
  void warmInstalledApplicationCache(language)
}

/**
 * Best-effort cache warming used by the shell's fire-and-forget prefetch.
 * Unsupported platforms legitimately reject application enumeration; that
 * must not escape as an unhandled promise rejection during application
 * startup. The privacy page performs its own guarded load and keeps the
 * native picker available when enumeration is unsupported.
 */
export async function warmInstalledApplicationCache(language: string): Promise<void> {
  try {
    const apps = await listInstalledApplications(language)
    installedCache = { language, apps }
    const missing = apps.filter((application) => !identityCache.has(application.id))
    for (let start = 0; start < missing.length; start += 32) {
      const batch = missing.slice(start, start + 32)
      try {
        const resolved = await describeApplications(batch.map((application) => application.id))
        for (const application of resolved) identityCache.set(application.id, application)
      } catch {
        // Icons are display data; a failed prefetch batch simply refetches later.
      }
    }
  } catch {
    // Cache warming is optional. In particular, Windows versions without the
    // enumeration capability return native_unavailable here by design.
  }
}

/**
 * The user-visible installed applications, as identifier/name pairs resolved
 * in the requested language (the frontend's active UI locale), so the grid
 * reads 备忘录 or Notes depending on what the user chose.
 *
 * Results are cached per language and warmed by prefetchInstalledApplications
 * at startup. Icons are deliberately not part of the listing: resolving every
 * icon up front is a heavy payload, and describeApplications supplies them
 * from its cache per batch. Platforms without the enumeration capability
 * reject the call — the caller keeps the native picker as the add path there.
 */
export async function listInstalledApplications(language: string): Promise<ApplicationDTO[]> {
  if (installedCache?.language === language) return installedCache.apps
  let apps: ApplicationDTO[]
  if (!('go' in window) || window.go === undefined) {
    apps = []
  } else {
    apps = (await ListInstalledApplications(language)) as unknown as ApplicationDTO[]
  }
  installedCache = { language, apps }
  return apps
}

/**
 * Resolves names and icons for the given identifiers, in input order. Cached
 * identities are returned without a binding round trip; only cache misses hit
 * the platform. This is the icon source for the installed-apps grid and
 * mirrors what getBlockedApplications uses, so both surfaces show identical
 * identities.
 */
export async function describeApplications(ids: string[]): Promise<ApplicationDTO[]> {
  if (ids.length === 0) return []
  const out: ApplicationDTO[] = []
  const missing: string[] = []
  for (const id of ids) {
    const cached = identityCache.get(id)
    if (cached) out.push(cached)
    else missing.push(id)
  }
  if (missing.length === 0) return out

  let resolvedBatch: ApplicationDTO[]
  if (!('go' in window) || window.go === undefined) {
    const names = (await getApplicationNamesDevelopmentFixture()) ?? {}
    resolvedBatch = missing.map((id) => ({ id, name: names[id] ?? '', iconDataUrl: '' }))
  } else {
    resolvedBatch = (await DescribeApplications(missing)) as unknown as ApplicationDTO[]
  }
  for (const application of resolvedBatch) identityCache.set(application.id, application)
  return [...out, ...resolvedBatch]
}
