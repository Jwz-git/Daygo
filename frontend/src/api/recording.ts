import { GetRecordingDirectory, GetRecordingState, PauseRecording, ResumeRecording, SetRecording } from '../../wailsjs/go/app/Backend'
import type { app } from '../../wailsjs/go/models'

export type RecordingState = app.RecordingStateDTO

export async function getRecordingState(): Promise<RecordingState> { return GetRecordingState() }
export async function setRecording(enabled: boolean): Promise<void> { return SetRecording(enabled) }
export async function pauseRecording(): Promise<void> { return PauseRecording(0) }
export async function resumeRecording(): Promise<void> { return ResumeRecording() }
export async function getRecordingDirectory(): Promise<string> { return GetRecordingDirectory() }
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
