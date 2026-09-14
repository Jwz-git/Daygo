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

/**
 * The user-visible installed applications, as identifier/name pairs resolved
 * in the requested language (the frontend's active UI locale), so the grid
 * reads 备忘录 or Notes depending on what the user chose.
 *
 * Icons are deliberately not part of the listing: resolving every icon up
 * front is a heavy payload, and the grid only needs icons for visible rows,
 * which describeApplications supplies per batch. Platforms without the
 * enumeration capability reject the call — the caller keeps the native
 * picker as the add path there.
 */
export async function listInstalledApplications(language: string): Promise<ApplicationDTO[]> {
  if (!('go' in window) || window.go === undefined) return []
  return (await ListInstalledApplications(language)) as unknown as ApplicationDTO[]
}

/**
 * Resolves names and icons for the given identifiers, in input order. This is
 * the icon source for the installed-apps grid and mirrors what
 * getBlockedApplications uses, so both surfaces show identical identities.
 */
export async function describeApplications(ids: string[]): Promise<ApplicationDTO[]> {
  if (ids.length === 0) return []
  if (!('go' in window) || window.go === undefined) {
    const names = (await getApplicationNamesDevelopmentFixture()) ?? {}
    return ids.map((id) => ({ id, name: names[id] ?? '', iconDataUrl: '' }))
  }
  return (await DescribeApplications(ids)) as unknown as ApplicationDTO[]
}
