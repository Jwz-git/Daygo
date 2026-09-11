import { GetDiagnostics } from '../../wailsjs/go/app/Backend'
import type { app } from '../../wailsjs/go/models'

export type DiagnosticsDTO = app.DiagnosticsDTO

export const DIAGNOSTICS_UNAVAILABLE = 'diagnostics_unavailable'
export async function getDiagnostics(): Promise<DiagnosticsDTO> {
  if (!('go' in window) || window.go === undefined) {
    throw new Error(DIAGNOSTICS_UNAVAILABLE)
  }
  return GetDiagnostics()
}
