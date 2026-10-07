import { GetDiagnostics } from '../../wailsjs/go/app/Backend'
import type { app } from '../../wailsjs/go/models'
import type { WireDTO } from '@/api/dto'

// Wails generates Go pointers as optional fields, although JSON sends null.
// Normalize those fields once at the boundary so consumers see the real shape.
export type DiagnosticsDTO = Omit<WireDTO<app.DiagnosticsDTO>, 'lastCaptureAtTs' | 'captureOwnerPid' | 'storageHealth'> & {
  lastCaptureAtTs: number | null
  captureOwnerPid: number | null
  storageHealth: WireDTO<app.StorageHealthDTO> | null
}

export const DIAGNOSTICS_UNAVAILABLE = 'diagnostics_unavailable'
export async function getDiagnostics(): Promise<DiagnosticsDTO> {
  if (!('go' in window) || window.go === undefined) {
    throw new Error(DIAGNOSTICS_UNAVAILABLE)
  }
  const result = await GetDiagnostics()
  return {
    ...result,
    lastCaptureAtTs: result.lastCaptureAtTs ?? null,
    captureOwnerPid: result.captureOwnerPid ?? null,
    storageHealth: result.storageHealth ?? null,
  }
}
