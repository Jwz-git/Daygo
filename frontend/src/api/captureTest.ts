import { hasDesktopBridge } from '@/api/desktopBridge'
import { CaptureTest, OpenCaptureTestFolder, PollSystemEvents } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'
import type * as app from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'

export type CaptureTestRequest = app.CaptureTestRequestDTO
export type CaptureTestResult = app.CaptureTestResultDTO
export type SystemEventTest = app.SystemEventTestDTO
export const WAILS_UNAVAILABLE = 'wails_unavailable'
export async function captureTest(request: CaptureTestRequest): Promise<CaptureTestResult> { if (!hasDesktopBridge()) throw new Error(WAILS_UNAVAILABLE); return CaptureTest(request) }
export async function openCaptureTestFolder(path: string): Promise<void> { if (!hasDesktopBridge()) throw new Error(WAILS_UNAVAILABLE); return OpenCaptureTestFolder(path) }
export async function pollSystemEvents(): Promise<SystemEventTest[]> { if (!hasDesktopBridge()) throw new Error(WAILS_UNAVAILABLE); return (await PollSystemEvents()) ?? [] }
