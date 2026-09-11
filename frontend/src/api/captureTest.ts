import { CaptureTest, OpenCaptureTestFolder } from '../../wailsjs/go/app/Backend'
import type { app } from '../../wailsjs/go/models'

export type CaptureTestRequest = app.CaptureTestRequestDTO
export type CaptureTestResult = app.CaptureTestResultDTO

export const WAILS_UNAVAILABLE = 'wails_unavailable'

export async function captureTest(request: CaptureTestRequest): Promise<CaptureTestResult> {
  if (!('go' in window) || window.go === undefined) {
    throw new Error(WAILS_UNAVAILABLE)
  }
  return CaptureTest(request)
}

export async function openCaptureTestFolder(path: string): Promise<void> {
  if (!('go' in window) || window.go === undefined) {
    throw new Error(WAILS_UNAVAILABLE)
  }
  return OpenCaptureTestFolder(path)
}
