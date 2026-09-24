import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import {
  getRecordingState,
  onRecordingState,
  pauseRecording,
  resumeRecording,
  setRecording,
  type RecordingState,
} from '@/api/recording'
import {
  getPermissionState,
  openSystemSettings,
  relaunchForPermission as requestPermissionRelaunch,
  requestScreenRecordingPermission,
  setPermissionRestartArmed,
} from '@/api/system'

export type RecordingLifecycle = 'idle' | 'starting' | 'capturing' | 'paused'
export type RecordingAction = 'start' | 'pause' | 'resume' | 'stop'

const LIFECYCLES: readonly RecordingLifecycle[] = ['idle', 'starting', 'capturing', 'paused']

export function lifecycleOf(value: string | undefined): RecordingLifecycle | null {
  return LIFECYCLES.includes(value as RecordingLifecycle) ? value as RecordingLifecycle : null
}

export const useRecordingStore = defineStore('recording', () => {
  const snapshot = ref<RecordingState | null>(null)
  const loading = ref(false)
  const pendingAction = ref<RecordingAction | null>(null)
  const error = ref<string | null>(null)
  const lifecycle = computed(() => lifecycleOf(snapshot.value?.state))
  const canControl = computed(() => snapshot.value?.isCaptureOwner === true)
  // Set when a start was blocked because screen recording is not authorized.
  // The UI shows the grant-and-restart guidance while this is true.
  const permissionRequired = ref(false)

  let stopEvents: (() => void) | null = null
  let requestVersion = 0

  /*
   * Show or hide the grant-and-restart guidance, keeping the native
   * permission-restart intent in sync. Arming while the guidance is up makes
   * the next quit — Cmd+Q, Dock, or macOS's own "Quit & Reopen" prompt after
   * the user flips the grant — a full terminate-and-relaunch rather than the
   * resident agent's soft-quit-to-background. The native flag is process-local
   * and resets on relaunch, so a failed sync (no Wails host, shell gone) is
   * safe to ignore.
   */
  function setPermissionGuidance(required: boolean): void {
    permissionRequired.value = required
    setPermissionRestartArmed(required).catch(() => {})
  }

  /*
   * Gate a start on screen-recording authorization. Granted lets the start
   * through. Not granted fires the system prompt (a first-use dialog, or a
   * no-op once denied) and raises permissionRequired so the UI can point the
   * user at System Settings — the grant only applies after a relaunch, so
   * there is no in-session success to wait for. A platform without the query
   * (Windows, or a plain browser) is not gated: the backend already reports
   * whether it can control capture.
   */
  async function ensureScreenRecordingPermission(): Promise<boolean> {
    let granted: boolean
    try {
      const state = await getPermissionState()
      granted = state.screenRecording === 'granted'
    } catch {
      return true
    }
    if (granted) {
      setPermissionGuidance(false)
      return true
    }
    try {
      await requestScreenRecordingPermission()
    } catch {
      // The prompt could not be shown; the guidance dialog still explains the
      // manual path through System Settings.
    }
    setPermissionGuidance(true)
    return false
  }

  async function openScreenRecordingSettings(): Promise<void> {
    try {
      await openSystemSettings('screen_recording')
    } catch {
      // Best effort: outside a Wails host there is no System Settings to open.
    }
  }

  /*
   * Fully quit and relaunch to apply a newly granted permission. On success the
   * process exits and a fresh instance starts, so there is nothing to refresh;
   * a failure (no desktop shell) surfaces through error.
   */
  async function relaunchForPermission(): Promise<void> {
    try {
      await requestPermissionRelaunch()
    } catch (cause: unknown) {
      error.value = cause instanceof Error ? cause.message : String(cause)
    }
  }

  function dismissPermissionPrompt(): void {
    setPermissionGuidance(false)
  }

  async function refresh(): Promise<void> {
    const version = ++requestVersion
    loading.value = snapshot.value === null
    try {
      const next = await getRecordingState()
      if (version !== requestVersion) return
      snapshot.value = next
      error.value = null
    } catch (cause: unknown) {
      if (version !== requestVersion) return
      error.value = cause instanceof Error ? cause.message : String(cause)
    } finally {
      if (version === requestVersion) loading.value = false
    }
  }

  function startListening(): void {
    if (stopEvents !== null) return
    stopEvents = onRecordingState(() => { void refresh() })
    void refresh()
  }

  function stopListening(): void {
    stopEvents?.()
    stopEvents = null
  }

  async function perform(action: RecordingAction): Promise<void> {
    if (pendingAction.value !== null || !canControl.value) return
    if (action === 'start' && !(await ensureScreenRecordingPermission())) return
    pendingAction.value = action
    error.value = null
    try {
      if (action === 'start') await setRecording(true)
      if (action === 'pause') await pauseRecording()
      if (action === 'resume') await resumeRecording()
      if (action === 'stop') await setRecording(false)
      await refresh()
    } catch (cause: unknown) {
      const actionError = cause instanceof Error ? cause.message : String(cause)
      await refresh()
      error.value = actionError
    } finally {
      pendingAction.value = null
    }
  }

  return {
    snapshot,
    lifecycle,
    loading,
    pendingAction,
    error,
    canControl,
    permissionRequired,
    refresh,
    startListening,
    stopListening,
    perform,
    openScreenRecordingSettings,
    relaunchForPermission,
    dismissPermissionPrompt,
  }
})
