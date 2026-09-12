import { GetBlockedApplications, PickApplication } from '../../wailsjs/go/app/Backend'

import { getApplicationNamesDevelopmentFixture } from '@/api/developmentFixtures'
import { WAILS_UNAVAILABLE, getSettings } from '@/api/settings'

/**
 * ApplicationDTO mirrors docs/05 §5.5.2: the display identity of one
 * application bundle. `name` and `iconDataUrl` are display data resolved by the
 * native layer; the persisted privacy setting holds only `id`. A name that came
 * back empty means the platform could not resolve the bundle, and the caller
 * shows `id` instead of inventing a label.
 */
export interface ApplicationDTO {
  id: string
  name: string
  iconDataUrl: string
}

/**
 * Opens the native macOS application picker. Returns null when the user
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
