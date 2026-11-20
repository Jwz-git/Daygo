import { hasDesktopBridge } from '@/api/desktopBridge'
import { GetDiagnostics } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'
import type * as app from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'
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
  if (!hasDesktopBridge()) {
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
