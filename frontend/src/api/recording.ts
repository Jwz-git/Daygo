import { CancelRecordingDirectoryMove, GetRecordingDirectory, GetRecordingDirectoryMigration, GetRecordingState, MoveRecordingDirectory, PauseRecording, PickRecordingDirectory, ResumeRecording, SetRecording, SetStatusItemLabels } from '../../wailsjs/go/app/Backend'
import type { app } from '../../wailsjs/go/models'

export type RecordingState = app.RecordingStateDTO
export type StatusItemLabels = app.StatusItemLabelsDTO

export async function getRecordingState(): Promise<RecordingState> { return GetRecordingState() }
export async function setRecording(enabled: boolean): Promise<void> { return SetRecording(enabled) }
export async function pauseRecording(): Promise<void> { return PauseRecording(0) }
export async function resumeRecording(): Promise<void> { return ResumeRecording() }
export async function getRecordingDirectory(): Promise<string> { return GetRecordingDirectory() }
export interface RecordingDirectoryMigration { source: string; target: string; phase: string; available: boolean }
export async function getRecordingDirectoryMigration(): Promise<RecordingDirectoryMigration> { return GetRecordingDirectoryMigration() }
export async function pickRecordingDirectory(): Promise<string> { return PickRecordingDirectory() }
export async function moveRecordingDirectory(path: string): Promise<void> { return MoveRecordingDirectory(path) }
export async function cancelRecordingDirectoryMove(): Promise<void> { return CancelRecordingDirectoryMove() }

// setStatusItemLabels pushes the localized menu-bar strings to the backend,
// which forwards them to the native status item. The item lives outside the
// webview, so vue-i18n cannot reach it directly. Best-effort: outside a Wails
// host (standalone preview) the generated binding is absent, so the failure is
// swallowed rather than surfaced.
export async function setStatusItemLabels(labels: StatusItemLabels): Promise<void> {
  try {
    await SetStatusItemLabels(labels)
  } catch {
    // No native host, or the status item is unavailable on this platform.
  }
}
interface RecordingEventPayload { state?: unknown }
interface RecordingRuntime { EventsOnMultiple?: (name: string, callback: (...data: unknown[]) => void, maxCallbacks: number) => (() => void) | undefined }
export function onRecordingState(callback: (state: string) => void): () => void {
  const runtime = (window as { runtime?: RecordingRuntime }).runtime
  if (typeof runtime?.EventsOnMultiple !== 'function') return () => undefined
  return runtime.EventsOnMultiple('recording:state', (...data: unknown[]) => {
    const value = data[0]
    if (typeof value !== 'object' || value === null || !('state' in value)) return
    const state = (value as RecordingEventPayload).state
    if (typeof state === 'string') callback(state)
  }, -1) ?? (() => undefined)
} 
