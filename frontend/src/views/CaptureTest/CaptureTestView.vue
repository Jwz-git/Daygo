<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import { captureTest, openCaptureTestFolder, pickCaptureTestApplication, pollSystemEvents, WAILS_UNAVAILABLE, type CaptureTestResult, type SystemEventTest } from '@/api/captureTest'
import { getRecordingState, setRecording } from '@/api/recording'

const { t } = useI18n()
const recordingState = ref('idle')
const recordingError = ref('')

const outputDirectory = ref('/tmp/daygo-capture-test')
const filenamePrefix = ref('daygo-capture')
const targetHeight = ref(720)
const jpegQuality = ref(85)
const showsCursor = ref(false)
const blockedApplicationIdsText = ref('')
const intervalSeconds = ref(0)
const durationSeconds = ref(60)
const running = ref(false)
const busy = ref(false)
const error = ref('')
const selectingApplication = ref(false)
const selectedApplicationLabel = ref('')
const results = ref<CaptureTestResult[]>([])
let timer: ReturnType<typeof setInterval> | undefined
let stopTimer: ReturnType<typeof setTimeout> | undefined
const systemEvents = ref<SystemEventTest[]>([])
let eventTimer: ReturnType<typeof setInterval> | undefined
const systemPaused = ref(false)

const blockedApplicationIds = computed(() =>
  blockedApplicationIdsText.value
    .split(/[,\n]/u)
    .map((value) => value.trim())
    .filter((value) => value.length > 0),
)

function request() {
  return {
    outputDirectory: outputDirectory.value.trim(),
    filenamePrefix: filenamePrefix.value.trim(),
    targetHeight: targetHeight.value,
    jpegQuality: jpegQuality.value,
    showsCursor: showsCursor.value,
    blockedApplicationIds: blockedApplicationIds.value,
  }
}
async function chooseBlockedApplication(): Promise<void> {
  if (selectingApplication.value || running.value) return
  selectingApplication.value = true
  error.value = ''
  try {
    const application = await pickCaptureTestApplication()
    if (application === null) return
    if (!blockedApplicationIds.value.includes(application.id)) {
      const current = blockedApplicationIdsText.value.trim()
      blockedApplicationIdsText.value = current === '' ? application.id : `${current}\n${application.id}`
    }
    selectedApplicationLabel.value = `${application.name} (${application.id})`
  } catch (cause: unknown) {
    error.value = cause instanceof Error && cause.message === WAILS_UNAVAILABLE
      ? t('captureTest.errors.wailsUnavailable')
      : cause instanceof Error ? cause.message : String(cause)
  } finally {
    selectingApplication.value = false
  }
}

async function captureOnce(): Promise<void> {
  if (busy.value || systemPaused.value) return
  busy.value = true
  error.value = ''
  try {
    const result = await captureTest(request())
    if (!systemPaused.value) results.value.unshift(result)
  } catch (cause: unknown) {
    error.value = cause instanceof Error && cause.message === WAILS_UNAVAILABLE ? t('captureTest.errors.wailsUnavailable') : cause instanceof Error ? cause.message : String(cause)
  } finally {
    busy.value = false
  }
}

function stopSchedule(): void {
  if (timer !== undefined) window.clearInterval(timer)
  if (stopTimer !== undefined) window.clearTimeout(stopTimer)
  timer = undefined
  stopTimer = undefined
  running.value = false
}

function startSchedule(): void {
  if (running.value || intervalSeconds.value < 1 || durationSeconds.value < 1) return
  running.value = true
  void captureOnce()
  timer = window.setInterval(() => void captureOnce(), intervalSeconds.value * 1000)
  stopTimer = window.setTimeout(stopSchedule, durationSeconds.value * 1000)
}

async function openFolder(): Promise<void> {
  const latest = results.value[0]
  if (!latest) return
  try {
    await openCaptureTestFolder(latest.outputPath)
  } catch (cause: unknown) {
    error.value = cause instanceof Error ? cause.message : String(cause)
  }
}
async function startRecording(): Promise<void> {
  recordingError.value = ''
  try { await setRecording(true); recordingState.value = (await getRecordingState()).state } catch (cause: unknown) { recordingError.value = cause instanceof Error ? cause.message : String(cause) }
}

function formatResult(result: CaptureTestResult): string {
  if (result.outcome === 'blocked') return t('captureTest.result.blocked')
  return t('captureTest.result.written', {
    width: result.width,
    height: result.height,
    bytes: result.fileSize,
  })
}

async function pollEvents(): Promise<void> {
  try {
    const events = await pollSystemEvents()
    if (events.length === 0) return
    systemEvents.value = [...events, ...systemEvents.value].slice(0, 32)
    for (const event of events) {
      if (['sleep', 'screen_locked', 'screensaver_start'].includes(event.kind)) {
        systemPaused.value = true
      } else if (['wake', 'screen_unlocked', 'screensaver_stop'].includes(event.kind)) {
        systemPaused.value = false
      }
    }
  } catch { /* Wails is optional in the Vite preview */ }
}

eventTimer = window.setInterval(() => void pollEvents(), 500)
void pollEvents()

onBeforeUnmount(() => {
  stopSchedule()
  if (eventTimer !== undefined) window.clearInterval(eventTimer)
})
</script>

<template>
  <div class="page capture-test">
    <PageHeader :title="t('captureTest.title')">
      <template #lead><span class="dg-chip dg-chip--filled">{{ t('captureTest.badge') }}</span></template>
      <template #trail>
        <button v-if="recordingState === 'idle'" type="button" class="dg-chip dg-chip--filled" @click="startRecording">{{ t('captureTest.startRecording') }}</button>
        <span v-else class="development-badge">{{ recordingState }}</span>
        <span v-if="recordingError" class="error">{{ recordingError }}</span>
      </template>
    </PageHeader>

    <div class="content dg-scroll">
      <p class="intro">{{ t('captureTest.description') }}</p>

      <section class="dg-card form-card" :aria-label="t('captureTest.configuration')">
        <h2>{{ t('captureTest.configuration') }}</h2>
        <label class="field">
          <span class="dg-field-label">{{ t('captureTest.outputDirectory') }}</span>
          <input v-model="outputDirectory" class="dg-input" type="text" spellcheck="false">
        </label>
        <label class="field">
          <span class="dg-field-label">{{ t('captureTest.filenamePrefix') }}</span>
          <input v-model="filenamePrefix" class="dg-input" type="text" spellcheck="false">
        </label>
        <div class="grid">
          <label class="field">
            <span class="dg-field-label">{{ t('captureTest.targetHeight') }}</span>
            <input v-model.number="targetHeight" class="dg-input" type="number" min="1" max="16384" step="1">
          </label>
          <label class="field">
            <span class="dg-field-label">{{ t('captureTest.jpegQuality') }}</span>
            <input v-model.number="jpegQuality" class="dg-input" type="number" min="1" max="100" step="1">
          </label>
        </div>
        <label class="check">
          <input v-model="showsCursor" type="checkbox">
          <span>{{ t('captureTest.showsCursor') }}</span>
        </label>
        <div class="field">
          <span class="dg-field-label">{{ t('captureTest.blockedApplicationIds') }}</span>
          <textarea v-model="blockedApplicationIdsText" class="dg-input textarea" rows="3" :placeholder="t('captureTest.blockedPlaceholder')" spellcheck="false" />
          <div class="application-selection">
            <button class="dg-button" :disabled="selectingApplication || running" type="button" @click="chooseBlockedApplication">
              {{ selectingApplication ? t('captureTest.selectingApplication') : t('captureTest.chooseApplication') }}
            </button>
            <span v-if="selectedApplicationLabel" class="hint">{{ selectedApplicationLabel }}</span>
          </div>
        </div>
      </section>

      <section class="dg-card form-card" :aria-label="t('captureTest.schedule')">
        <h2>{{ t('captureTest.schedule') }}</h2>
        <div class="grid">
          <label class="field">
            <span class="dg-field-label">{{ t('captureTest.intervalSeconds') }}</span>
            <input v-model.number="intervalSeconds" class="dg-input" type="number" min="0" step="1">
          </label>
          <label class="field">
            <span class="dg-field-label">{{ t('captureTest.durationSeconds') }}</span>
            <input v-model.number="durationSeconds" class="dg-input" type="number" min="1" step="1">
          </label>
        </div>
        <p class="hint">{{ t('captureTest.scheduleHint') }}</p>
        <div class="actions">
          <button class="dg-button dg-button--primary" :disabled="busy || running" type="button" @click="captureOnce">
            {{ t('captureTest.captureOnce') }}
          </button>
          <button class="dg-button" :disabled="running" type="button" @click="startSchedule">
            {{ t('captureTest.startSchedule') }}
          </button>
          <button class="dg-button" :disabled="results.length === 0" type="button" @click="openFolder">
            {{ t('captureTest.openFolder') }}
          </button>
          <button class="dg-button" :disabled="!running" type="button" @click="stopSchedule">
            {{ t('captureTest.stopSchedule') }}
          </button>
        </div>
      </section>
      <section class="dg-card results" :aria-label="t('captureTest.systemEvents')">
        <h2>{{ t('captureTest.systemEvents') }}</h2>
        <p class="hint">{{ t('captureTest.systemEventsHint') }}</p>
        <ol v-if="systemEvents.length > 0">
          <li v-for="event in systemEvents" :key="`${event.kind}-${event.atTs}`"><strong>{{ event.kind }}</strong> <code>{{ event.atTs }}</code></li>
        </ol>
        <p v-else class="hint">{{ t('captureTest.noSystemEvents') }}</p>
      </section>

      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <section v-if="results.length > 0" class="dg-card results" :aria-label="t('captureTest.results')">
        <h2>{{ t('captureTest.results') }}</h2>
        <ol>
          <li v-for="result in results" :key="result.outputPath">
            <strong>{{ formatResult(result) }}</strong>
            <code>{{ result.outputPath }}</code>
          </li>
        </ol>
      </section>
    </div>
  </div>
</template>

<style scoped>
.content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

.intro,
.hint {
  color: var(--dg-text-secondary);
  font-size: 13px;
  line-height: 1.5;
}

.dg-card {
  padding: 20px;
}

h2 {
  margin: 0 0 18px;
  color: var(--dg-text-primary);
  font-size: 15px;
}

.form-card {
  display: grid;
  gap: 16px;
}

.field {
  display: grid;
  gap: 7px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.textarea {
  resize: vertical;
}
.application-selection {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}


.check {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--dg-text-secondary);
  font-size: 13px;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.error {
  padding: 12px 14px;
  border: 1px solid var(--dg-danger);
  border-radius: 10px;
  color: var(--dg-danger);
  font-size: 13px;
}

.results ol {
  display: grid;
  gap: 12px;
  margin: 0;
  padding-left: 20px;
}

.results li {
  display: grid;
  gap: 4px;
  color: var(--dg-text-secondary);
  font-size: 13px;
}

.results strong {
  color: var(--dg-text-primary);
}

.results code {
  overflow-wrap: anywhere;
  color: var(--dg-text-tertiary);
  font-family: var(--dg-font-mono);
  font-size: 12px;
}

@media (max-width: 680px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
