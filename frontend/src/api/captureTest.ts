import { CaptureTest, OpenCaptureTestFolder, PickCaptureTestApplication } from '../../wailsjs/go/app/Backend'
import type { app } from '../../wailsjs/go/models'

export type CaptureTestRequest = app.CaptureTestRequestDTO
export type CaptureTestResult = app.CaptureTestResultDTO
export type CaptureTestApplication = app.CaptureTestApplicationDTO


export const WAILS_UNAVAILABLE = 'wails_unavailable'

export async function captureTest(request: CaptureTestRequest): Promise<CaptureTestResult> {
  if (!('go' in window) || window.go === undefined) {
    throw new Error(WAILS_UNAVAILABLE)
  }
  return CaptureTest(request)
}
export async function pickCaptureTestApplication(): Promise<CaptureTestApplication | null> {
  if (!('go' in window) || window.go === undefined) {
    throw new Error(WAILS_UNAVAILABLE)
  }
  return PickCaptureTestApplication()
}


export async function openCaptureTestFolder(path: string): Promise<void> {
  if (!('go' in window) || window.go === undefined) {
    throw new Error(WAILS_UNAVAILABLE)
  }
  return OpenCaptureTestFolder(path)
}
