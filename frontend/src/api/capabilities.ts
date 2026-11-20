import { hasDesktopBridge } from '@/api/desktopBridge'
import { GetCapabilities } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'
import type { CapabilitiesDTO } from '@/api/dto'

export async function getCapabilities(): Promise<CapabilitiesDTO | null> {
  if (!hasDesktopBridge()) return null
  const value = await GetCapabilities()
  return { ...value, features: value.features ?? [] }
}
