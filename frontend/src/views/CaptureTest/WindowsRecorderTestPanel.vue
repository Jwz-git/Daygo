<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { openCaptureTestFolder } from '@/api/captureTest'
import { CAPTURE_HEIGHTS, CAPTURE_INTERVAL_SECONDS } from '@/api/dto'
import {
  getRecordingDirectory,
  getRecordingState,
  onRecordingState,
  pauseRecording,
  resumeRecording,
  setRecording,
  type RecordingState,
} from '@/api/recording'
import { getSettings, updateSettings } from '@/api/settings'

const { t } = useI18n()
const state = ref<RecordingState | null>(null)
const directory = ref('')
const intervalSeconds = ref(10)
const targetHeight = ref(720)
const durationSeconds = ref(60)
const blockedApplicationCount = ref(0)
const capturesObserved = ref(0)
const lastObservedFrame = ref<number | null>(null)
const error = ref('')
const busy = ref(false)
const startedHere = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | undefined
let stopTimer: ReturnType<typeof setTimeout> | undefined
let unsubscribe: () => void = () => undefined

const lifecycle = computed(() => state.value?.state ?? 'idle')
const running = computed(() => lifecycle.value !== 'idle')
const canStart = computed(() => !busy.value && lifecycle.value === 'idle' && blockedApplicationCount.value === 0)

function errorMessage(cause: unknown): string {
  return cause instanceof Error ? cause.message : String(cause)
}

async function refreshState(): Promise<void> {
  try {
    const next = await getRecordingState()
    const frame = next.lastFrameAtTs ?? null
    if (startedHere.value && frame !== null && frame !== lastObservedFrame.value) capturesObserved.value += 1
    lastObservedFrame.value = frame
    state.value = next
  } catch (cause: unknown) {
    error.value = errorMessage(cause)
  }
}

async function load(): Promise<void> {
  try {
    const [settings, recordingDirectory] = await Promise.all([getSettings(), getRecordingDirectory()])
    intervalSeconds.value = settings.capture.intervalSeconds
    targetHeight.value = settings.capture.captureHeight
    blockedApplicationCount.value = settings.privacy.blockedApplicationIds.length
    directory.value = recordingDirectory
    await refreshState()
  } catch (cause: unknown) {
    error.value = errorMessage(cause)
  }
}

async function start(): Promise<void> {
  if (!canStart.value || durationSeconds.value < 1) return
  busy.value = true
  error.value = ''
  capturesObserved.value = 0
  try {
    const current = await getSettings()
    blockedApplicationCount.value = current.privacy.blockedApplicationIds.length
    if (blockedApplicationCount.value > 0) {
      error.value = t('captureTest.windowsPrivacyMustBeEmpty')
      return
    }
    await updateSettings({ intervalSeconds: intervalSeconds.value, captureHeight: targetHeight.value })
    lastObservedFrame.value = (await getRecordingState()).lastFrameAtTs ?? null
    startedHere.value = true
    await setRecording(true)
    await refreshState()
    stopTimer = window.setTimeout(() => void stop(), durationSeconds.value * 1000)
  } catch (cause: unknown) {
    startedHere.value = false
    error.value = errorMessage(cause)
  } finally {
    busy.value = false
  }
}

async function stop(): Promise<void> {
  if (stopTimer !== undefined) window.clearTimeout(stopTimer)
  stopTimer = undefined
  busy.value = true
  error.value = ''
  try {
    await setRecording(false)
    await refreshState()
  } catch (cause: unknown) {
    error.value = errorMessage(cause)
  } finally {
    busy.value = false
    startedHere.value = false
  }
}

async function pause(): Promise<void> {
  busy.value = true
  error.value = ''
  try { await pauseRecording(); await refreshState() }
  catch (cause: unknown) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}

async function resume(): Promise<void> {
  busy.value = true
  error.value = ''
  try { await resumeRecording(); await refreshState() }
  catch (cause: unknown) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}

async function openFolder(): Promise<void> {
  if (directory.value === '') return
  const separator = directory.value.includes('\\') ? '\\' : '/'
  try {
    await openCaptureTestFolder(`${directory.value}${separator}staging${separator}capture.jpg`)
  } catch (cause: unknown) {
    error.value = errorMessage(cause)
  }
}

onMounted(() => {
  void load()
  unsubscribe = onRecordingState(() => void refreshState())
  refreshTimer = window.setInterval(() => void refreshState(), 500)
})

onBeforeUnmount(() => {
  unsubscribe()
  if (refreshTimer !== undefined) window.clearInterval(refreshTimer)
  if (stopTimer !== undefined) window.clearTimeout(stopTimer)
  if (startedHere.value) void setRecording(false)
})
</script>

<template>
  <p class="intro">{{ t('captureTest.windowsDescription') }}</p>

  <section class="dg-card form-card">
    <h2>{{ t('captureTest.windowsRecorderConfiguration') }}</h2>
    <div class="grid">
      <label class="field">
        <span class="dg-field-label">{{ t('captureTest.intervalSeconds') }}</span>
        <select v-model.number="intervalSeconds" class="dg-input" :disabled="running">
          <option v-for="value in CAPTURE_INTERVAL_SECONDS" :key="value" :value="value">{{ value }}</option>
        </select>
      </label>
      <label class="field">
        <span class="dg-field-label">{{ t('captureTest.targetHeight') }}</span>
        <select v-model.number="targetHeight" class="dg-input" :disabled="running">
          <option v-for="value in CAPTURE_HEIGHTS" :key="value" :value="value">{{ value }}</option>
        </select>
      </label>
      <label class="field">
        <span class="dg-field-label">{{ t('captureTest.durationSeconds') }}</span>
        <input v-model.number="durationSeconds" class="dg-input" type="number" min="1" step="1" :disabled="running">
      </label>
    </div>
    <p class="hint">{{ t('captureTest.windowsScheduleHint') }}</p>
    <p v-if="blockedApplicationCount > 0" class="warning">
      {{ t('captureTest.windowsPrivacyConfigured', { count: blockedApplicationCount }) }}
    </p>
    <div class="actions">
      <button class="dg-button dg-button--primary" type="button" :disabled="!canStart" @click="start">{{ t('captureTest.startRecorder') }}</button>
      <button class="dg-button" type="button" :disabled="busy || lifecycle !== 'capturing'" @click="pause">{{ t('captureTest.pauseRecorder') }}</button>
      <button class="dg-button" type="button" :disabled="busy || lifecycle !== 'paused'" @click="resume">{{ t('captureTest.resumeRecorder') }}</button>
      <button class="dg-button" type="button" :disabled="busy || !running" @click="stop">{{ t('captureTest.stopRecorder') }}</button>
      <button class="dg-button" type="button" :disabled="capturesObserved === 0" @click="openFolder">{{ t('captureTest.openFolder') }}</button>
    </div>
  </section>

  <section class="dg-card status-card">
    <h2>{{ t('captureTest.windowsRecorderStatus') }}</h2>
    <dl>
      <div><dt>{{ t('captureTest.lifecycleState') }}</dt><dd><code>{{ lifecycle }}</code></dd></div>
      <div><dt>{{ t('captureTest.permissionState') }}</dt><dd><code>{{ state?.permission ?? '—' }}</code></dd></div>
      <div><dt>{{ t('captureTest.capturesObserved') }}</dt><dd>{{ capturesObserved }}</dd></div>
      <div><dt>{{ t('captureTest.lastFrameAt') }}</dt><dd>{{ state?.lastFrameAtTs ? new Date(state.lastFrameAtTs * 1000).toLocaleString() : '—' }}</dd></div>
      <div><dt>{{ t('captureTest.recordingDirectory') }}</dt><dd><code>{{ directory || '—' }}</code></dd></div>
    </dl>
    <p class="hint">{{ t('captureTest.windowsMocksHint') }}</p>
  </section>

  <p v-if="error" class="error" role="alert">{{ error }}</p>
</template>

<style scoped>
.intro,.hint{color:var(--dg-text-secondary);font-size:13px;line-height:1.5}.dg-card{padding:20px}h2{margin:0 0 18px;color:var(--dg-text-primary);font-size:15px}.form-card,.field{display:grid;gap:16px}.field{gap:7px}.grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.actions{display:flex;flex-wrap:wrap;gap:10px}.warning,.error{padding:12px 14px;border-radius:10px;font-size:13px}.warning{border:1px solid var(--dg-warning);color:var(--dg-text-primary)}.error{border:1px solid var(--dg-danger);color:var(--dg-danger)}dl{display:grid;gap:10px;margin:0}dl div{display:grid;grid-template-columns:160px minmax(0,1fr);gap:12px}dt{color:var(--dg-text-secondary);font-size:13px}dd{margin:0;color:var(--dg-text-primary);overflow-wrap:anywhere}code{font-family:var(--dg-font-mono);font-size:12px}@media(max-width:680px){.grid{grid-template-columns:1fr}dl div{grid-template-columns:1fr}}
</style>
