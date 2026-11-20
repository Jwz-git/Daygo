import { desktopEnvironment, hasDesktopBridge } from '@/api/desktopBridge'

export type DesktopPlatform = 'darwin' | 'windows' | 'linux' | 'browser' | 'unknown'

export function parseDesktopPlatform(value: unknown): DesktopPlatform {
  if (typeof value !== 'object' || value === null || !('platform' in value)) return 'unknown'
  switch (value.platform) {
    case 'darwin':
    case 'windows':
    case 'linux':
      return value.platform
    default:
      return 'unknown'
  }
}

export async function resolveDesktopPlatform(): Promise<DesktopPlatform> {
  if (!hasDesktopBridge()) return 'browser'

  try {
    return parseDesktopPlatform(await desktopEnvironment())
  } catch {
    return 'unknown'
  }
}
