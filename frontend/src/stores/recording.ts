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

  let stopEvents: (() => void) | null = null
  let requestVersion = 0

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
    refresh,
    startListening,
    stopListening,
    perform,
  }
})
