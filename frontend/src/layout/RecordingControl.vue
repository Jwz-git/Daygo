<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'

import { useRecordingStore, type RecordingAction } from '@/stores/recording'

const store = useRecordingStore()
const { lifecycle, loading, pendingAction, error, canControl, snapshot } = storeToRefs(store)
const { t } = useI18n()
const open = ref(false)

const stateKey = computed(() => loading.value ? 'loading' : lifecycle.value ?? 'unknown')
const actions = computed<readonly RecordingAction[]>(() => {
  if (lifecycle.value === 'idle') return ['start']
  if (lifecycle.value === 'paused') return ['resume', 'stop']
  if (lifecycle.value === 'capturing') return ['pause', 'stop']
  return []
})

function choose(action: RecordingAction): void {
  void store.perform(action)
  if (action === 'stop' || action === 'start') open.value = false
}
</script>

<template>
  <div class="recording-control" @keydown.esc="open = false">
    <button
      type="button"
      class="recording-control__trigger"
      :class="`is-${stateKey}`"
      :aria-expanded="open"
      :title="t(`recording.state.${stateKey}`)"
      @click="open = !open"
    >
      <span class="recording-control__dot" aria-hidden="true"></span>
      <span>{{ t(`recording.state.${stateKey}`) }}</span>
    </button>

    <div v-if="open" class="recording-control__popover">
      <strong>{{ t(`recording.state.${stateKey}`) }}</strong>
      <p v-if="!canControl">{{ t('recording.notOwner') }}</p>
      <p v-else-if="snapshot?.permission !== 'granted'">{{ t('recording.permissionRequired') }}</p>
      <p v-else-if="error" role="alert">{{ t('recording.actionFailed') }}</p>
      <div class="recording-control__actions">
        <button
          v-for="action in actions"
          :key="action"
          type="button"
          class="dg-button"
          :class="{ 'dg-button--primary': action === 'start' || action === 'resume' }"
          :disabled="pendingAction !== null || !canControl"
          @click="choose(action)"
        >
          {{ pendingAction === action ? t('recording.working') : t(`recording.action.${action}`) }}
        </button>
        <button v-if="error" type="button" class="dg-button" @click="store.refresh">
          {{ t('common.action.retry') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.recording-control { position: relative; width: 100%; }
.recording-control__trigger {
  display: flex; flex-direction: column; align-items: center; gap: 6px;
  width: 100%; min-height: 52px; padding: 6px 2px;
  color: var(--dg-text-secondary); font-size: 11px; line-height: 1.1;
}
.recording-control__trigger:hover { color: var(--dg-text-primary); }
.recording-control__trigger:focus-visible { outline: none; }
.recording-control__trigger:focus-visible .recording-control__dot { box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.recording-control__dot {
  width: 10px; height: 10px; border: 2px solid var(--dg-text-muted);
  border-radius: 50%; background: transparent;
}
.is-capturing .recording-control__dot { border-color: var(--dg-success); background: var(--dg-success); }
.is-starting .recording-control__dot,
.is-loading .recording-control__dot { border-color: var(--dg-accent); background: var(--dg-accent); }
.is-paused .recording-control__dot { border-color: var(--dg-warning); background: var(--dg-warning); }
.recording-control__popover {
  position: absolute; z-index: 20; bottom: 0; left: calc(100% + 10px);
  width: 250px; padding: 14px; border: 1px solid var(--dg-panel-border);
  border-radius: 10px; background: var(--dg-popover-fill); box-shadow: var(--dg-popover-shadow);
  color: var(--dg-text-primary);
}
.recording-control__popover strong { font-size: 13px; }
.recording-control__popover p { margin-top: 6px; color: var(--dg-text-secondary); font-size: 12px; }
.recording-control__actions { display: flex; gap: 7px; margin-top: 12px; }
</style>
