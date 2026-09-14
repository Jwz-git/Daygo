<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'

import { useRecordingStore, type RecordingAction } from '@/stores/recording'

const store = useRecordingStore()
const { lifecycle, loading } = storeToRefs(store)
const { t } = useI18n()

const stateKey = computed(() => loading.value ? 'loading' : lifecycle.value ?? 'unknown')

const action = computed<RecordingAction | null>(() => {
  if (loading.value) return null
  if (lifecycle.value === 'idle') return 'start'
  if (lifecycle.value === 'capturing') return 'pause'
  if (lifecycle.value === 'paused') return 'resume'
  return null
})

function onTriggerClick(): void {
  const a = action.value
  if (a !== null) void store.perform(a)
}
</script>

<template>
  <div class="recording-control lg-tracking">
    <button
      type="button"
      class="recording-control__trigger"
      :class="`is-${stateKey}`"
      :title="t(`recording.state.${stateKey}`)"
      :disabled="action === null"
      @click="onTriggerClick"
    >
      <span class="recording-control__dot" aria-hidden="true" />
      <span>{{ t(`recording.state.${stateKey}`) }}</span>
    </button>
  </div>
</template>

<style scoped>
.recording-control { position: relative; width: 100%; }

.recording-control__trigger {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 52px;
  padding: 6px 2px;
  border: none;
  background: none;
  color: var(--dg-text-secondary);
  font-size: 11px;
  line-height: 1.1;
  cursor: pointer;
  transition: color var(--dg-motion-base) ease;
}

.recording-control__trigger:hover:not(:disabled) { color: var(--dg-text-primary); }

.recording-control__trigger:focus-visible { outline: none; }

.recording-control__trigger:focus-visible .recording-control__dot {
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.recording-control__trigger:disabled { opacity: 0.4; cursor: default; }

.recording-control__dot {
  width: 10px;
  height: 10px;
  border: 2px solid var(--dg-text-muted);
  border-radius: 50%;
  background: transparent;
  transition: border-color var(--dg-motion-base) ease, background var(--dg-motion-base) ease;
}

.is-capturing .recording-control__dot { border-color: var(--dg-success); background: var(--dg-success); }
.is-starting .recording-control__dot,
.is-loading .recording-control__dot { border-color: var(--dg-accent); background: var(--dg-accent); }
.is-paused .recording-control__dot { border-color: var(--dg-warning); background: var(--dg-warning); }
</style>
