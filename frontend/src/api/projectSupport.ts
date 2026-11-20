import { hasDesktopBridge, openDesktopURL } from '@/api/desktopBridge'

export const PROJECT_SUPPORT_URL = 'https://github.com/Jwz-git/Daygo'

/** Return false in browser previews so the anchor can use normal navigation. */
export function openProjectSupport(): boolean {
 if (!hasDesktopBridge()) return false
 openDesktopURL(PROJECT_SUPPORT_URL)
 return true
}
