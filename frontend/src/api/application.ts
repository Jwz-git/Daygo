import { hasDesktopBridge } from '@/api/desktopBridge'
import type * as app from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'
import type { WireDTO } from '@/api/dto'
import {
  DescribeApplications,
  GetBlockedApplications,
  GetPrivacyCompatibility,
  ListInstalledApplications,
  PickApplication,
} from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'

import { getApplicationNamesDevelopmentFixture } from '@/api/developmentFixtures'
import { WAILS_UNAVAILABLE, getSettings } from '@/api/settings'
import { LruCache } from '@/lib/lruCache'

/**
 * ApplicationDTO mirrors docs/05 §5.5.2: the display identity of one
 * application. `name` and `iconDataUrl` are display data resolved by the
 * native layer; the persisted privacy setting holds only `id`. A name that came
 * back empty means the platform could not resolve the bundle, and the caller
 * shows `id` instead of inventing a label.
 */
export type ApplicationDTO = WireDTO<app.ApplicationDTO>

export type PrivacyCompatibilityDTO = WireDTO<app.PrivacyCompatibilityDTO>

export async function getPrivacyCompatibility(): Promise<PrivacyCompatibilityDTO | null> {
  if (!hasDesktopBridge()) return null
  return GetPrivacyCompatibility()
}

/**
 * Opens the platform's native application picker. Returns null when the user
 * cancels. Outside the Wails app there is no picker and no stand-in, so this
 * reports the unavailable state rather than opening a text prompt.
 */
export async function pickApplication(): Promise<ApplicationDTO | null> {
  if (!hasDesktopBridge()) throw new Error(WAILS_UNAVAILABLE)
  return PickApplication()
}

/**
 * The configured privacy list in configuration order, with whatever display
 * identity the platform can resolve. The backend reads the identifiers itself,
 * so the browser stand-in is the only path that needs the dev fixture.
 */
export async function getBlockedApplications(): Promise<ApplicationDTO[]> {
  if (hasDesktopBridge()) {
    return (await GetBlockedApplications()) ?? []
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
 * Bounded per-language enumeration cache plus a bounded icon memo. The privacy
 * grid enumerates on mount and re-enumerates when the UI language changes, and
 * timeline icon matching may query both the active UI locale and English so
 * localized apps match English model labels without thrashing a single slot.
 */
const MAX_INSTALLED_LOCALE_CACHE = 4
const installedCache = new Map<string, ApplicationDTO[]>()
const pendingLists = new Map<string, Promise<ApplicationDTO[]>>()
const pendingIdentities = new Map<string, Promise<ApplicationDTO | undefined>>()
const identityCache = new LruCache<string, ApplicationDTO>(512, {
  maxWeight: 8 * 1024 * 1024,
  weigh: (value) => (value.id.length + value.name.length + value.iconDataUrl.length) * 2,
})

/**
 * The user-visible installed applications, as identifier/name pairs resolved
 * in the requested language (the frontend's active UI locale), so the grid
 * reads 备忘录 or Notes depending on what the user chose.
 *
 * Results are cached per language. Icons are deliberately not part of the
 * listing: resolving every icon up front is a heavy payload, and
 * describeApplications supplies them from its cache per batch. Platforms
 * without the enumeration capability reject the call — the caller keeps the
 * native picker as the add path there.
 */
export async function listInstalledApplications(language: string): Promise<ApplicationDTO[]> {
  const cached = installedCache.get(language)
  if (cached !== undefined) return cached
  const pending = pendingLists.get(language)
  if (pending !== undefined) return pending

  const request = (hasDesktopBridge() ? ListInstalledApplications(language) : Promise.resolve([]))
    .then((apps) => {
      const results = apps ?? []
      if (installedCache.size >= MAX_INSTALLED_LOCALE_CACHE) {
        const oldest = installedCache.keys().next().value
        if (oldest !== undefined) installedCache.delete(oldest)
      }
      installedCache.set(language, results)
      return results
    })
    .finally(() => pendingLists.delete(language))
  pendingLists.set(language, request)
  return request
}

/**
 * Resolves names and icons for the given identifiers, in input order. Cached
 * identities are returned without a binding round trip; only cache misses hit
 * the platform. This is the icon source for the installed-apps grid and
 * mirrors what getBlockedApplications uses, so both surfaces show identical
 * identities.
 */
export async function describeApplications(ids: string[]): Promise<ApplicationDTO[]> {
  const missing = [...new Set(ids.filter((id) => !identityCache.get(id) && !pendingIdentities.has(id)))]
  if (missing.length > 0) {
    const batch = loadIdentities(missing).then((apps) => {
      const byID = new Map(apps.map((app) => [app.id, app]))
      for (const app of apps) identityCache.set(app.id, app)
      return byID
    })
    for (const id of missing) {
      pendingIdentities.set(id, batch.then((apps) => apps.get(id)).finally(() => pendingIdentities.delete(id)))
    }
  }
  const found = await Promise.all(ids.map((id) => identityCache.get(id) ?? pendingIdentities.get(id)))
  return found.filter((item): item is ApplicationDTO => item !== undefined)
}

async function loadIdentities(ids: string[]): Promise<ApplicationDTO[]> {
  if (hasDesktopBridge()) return (await DescribeApplications(ids)) ?? []
  const names = (await getApplicationNamesDevelopmentFixture()) ?? {}
  return ids.map((id) => ({ id, name: names[id] ?? '', iconDataUrl: '' }))
}
