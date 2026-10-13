import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'

export const PROJECT_SUPPORT_URL = 'https://github.com/Jwz-git/Daygo'

/** Return false in browser previews so the anchor can use normal navigation. */
export function openProjectSupport(): boolean {
  const runtime: unknown = 'runtime' in window ? window.runtime : null
  if (typeof runtime !== 'object' || runtime === null ||
      !('BrowserOpenURL' in runtime) || typeof runtime.BrowserOpenURL !== 'function') return false
  BrowserOpenURL(PROJECT_SUPPORT_URL)
  return true
}
