import { GetCapabilities } from '../../wailsjs/go/app/Backend'
import type { CapabilitiesDTO } from '@/api/dto'

export async function getCapabilities(): Promise<CapabilitiesDTO | null> {
  if (!('go' in window) || window.go === undefined) return null
  return GetCapabilities()
}
