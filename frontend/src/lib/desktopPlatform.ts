import { Environment } from '../../wailsjs/runtime/runtime'

export type DesktopPlatform = 'darwin' | 'windows' | 'linux' | 'browser' | 'unknown'

interface RuntimeBridge {
  Environment: () => Promise<unknown>
}

function runtimeBridge(value: unknown): value is RuntimeBridge {
  return typeof value === 'object' && value !== null &&
    'Environment' in value && typeof value.Environment === 'function'
}

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
  const runtime = (window as { runtime?: unknown }).runtime
  if (!runtimeBridge(runtime)) return 'browser'

  try {
    return parseDesktopPlatform(await Environment())
  } catch {
    return 'unknown'
  }
}
