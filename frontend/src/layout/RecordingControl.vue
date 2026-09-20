<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'

import { useRecordingStore, type RecordingAction } from '@/stores/recording'

const store = useRecordingStore()
const { lifecycle, loading, permissionRequired } = storeToRefs(store)
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

function onOpenSettings(): void {
  void store.openScreenRecordingSettings()
}

function onDismiss(): void {
  store.dismissPermissionPrompt()
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

    <Teleport to="body">
      <div
        v-if="permissionRequired"
        class="recording-permission"
        role="dialog"
        aria-modal="true"
        :aria-label="t('recording.permission.title')"
      >
        <div class="recording-permission__backdrop" @click="onDismiss" />
        <div class="recording-permission__panel">
          <h2 class="recording-permission__title">{{ t('recording.permission.title') }}</h2>
          <p class="recording-permission__body">{{ t('recording.permission.body') }}</p>
          <ol class="recording-permission__steps">
            <li>{{ t('recording.permission.step1') }}</li>
            <li>{{ t('recording.permission.step2') }}</li>
            <li>{{ t('recording.permission.step3') }}</li>
          </ol>
          <div class="recording-permission__actions">
            <button type="button" class="recording-permission__btn is-secondary" @click="onDismiss">
              {{ t('recording.permission.dismiss') }}
            </button>
            <button type="button" class="recording-permission__btn is-primary" @click="onOpenSettings">
              {{ t('recording.permission.openSettings') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
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

.recording-permission {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
}

.recording-permission__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
}

.recording-permission__panel {
  position: relative;
  width: min(420px, calc(100vw - 48px));
  padding: 24px;
  border-radius: 14px;
  background: var(--dg-surface-raised, #1e1e22);
  color: var(--dg-text-primary);
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.4);
  text-align: left;
}

.recording-permission__title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
}

.recording-permission__body {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--dg-text-secondary);
}

.recording-permission__steps {
  margin: 0 0 20px;
  padding-left: 20px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--dg-text-secondary);
}

.recording-permission__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.recording-permission__btn {
  padding: 8px 16px;
  border: none;
  border-radius: 8px;
  font-size: 13px;
  cursor: pointer;
  transition: opacity var(--dg-motion-base) ease;
}

.recording-permission__btn:hover { opacity: 0.85; }

.recording-permission__btn.is-secondary {
  background: var(--dg-surface-sunken, rgba(255, 255, 255, 0.08));
  color: var(--dg-text-primary);
}

.recording-permission__btn.is-primary {
  background: var(--dg-accent);
  color: #fff;
}
</style>
