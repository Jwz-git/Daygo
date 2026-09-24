<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TimelineFailureDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import type { TimelineAction } from '@/stores/timeline'

/* The inspector's failure pane: what failed, why, and the batch-level
   retry/dismiss actions. */
const props = defineProps<{
  failure: TimelineFailureDTO
  canWrite: boolean
  actions: TimelineActionAvailability
  pendingAction: TimelineAction | null
  actionFailed: boolean
}>()

const emit = defineEmits<{
  close: []
  retry: [batchIDs: number[]]
  stopRetries: [batchIDs: number[]]
  dismiss: [batchIDs: number[]]
}>()

const { t, locale } = useI18n()
const confirmingDelete = ref(false)

const canRetry = computed(() => props.canWrite && props.actions.retryBatches)
const canDelete = computed(() => props.canWrite && props.actions.deleteBatches)
// Stopping only makes sense while the failure is still on the auto-retry
// track; once it is not retryable (kind needs attention, or already stopped)
// there is nothing to halt.
const canStop = computed(
  () => props.canWrite && props.actions.stopRetries && props.failure.retryable,
)

const failureClock = computed(() => {
  const format = new Intl.DateTimeFormat(locale.value, {
    hour: 'numeric',
    minute: '2-digit',
  })
  return `${format.format(new Date(props.failure.startTs * 1000))} – ${format.format(new Date(props.failure.endTs * 1000))}`
})

watch(
  () => props.failure.startTs,
  () => {
    confirmingDelete.value = false
  },
)
</script>

<template>
  <header class="inspector__header">
    <div>
      <p class="inspector__eyebrow inspector__eyebrow--danger">{{ t('timeline.failure.title') }}</p>
      <h2 class="inspector__title inspector__title--card">{{ t('timeline.failure.detailTitle') }}</h2>
    </div>
    <button
      type="button"
      class="inspector__close"
      :aria-label="t('timeline.inspector.close')"
      @click="emit('close')"
    >
      ×
    </button>
  </header>

  <div class="card-time card-time--failure">
    <span :style="{ background: 'var(--dg-danger)' }"></span>
    {{ failureClock }}
  </div>

  <section class="inspector__section">
    <h3>{{ t('timeline.failure.detail.kind') }}</h3>
    <p class="inspector__failure-kind">{{ props.failure.kind }}</p>
  </section>

  <section class="inspector__section">
    <h3>{{ t('timeline.failure.detail.message') }}</h3>
    <p class="inspector__failure-message">{{ props.failure.message }}</p>
  </section>

  <section class="inspector__section">
    <h3>{{ t('timeline.failure.detail.batches') }}</h3>
    <ul class="failure-batches">
      <li v-for="id in props.failure.batchIds" :key="id">#{{ id }}</li>
    </ul>
    <p class="inspector__failure-note">
      {{ props.failure.retryable ? t('timeline.failure.detail.retryable') : t('timeline.failure.detail.notRetryable') }}
    </p>
  </section>

  <p v-if="props.actionFailed" class="inspector__error" role="alert">
    {{ t('timeline.inspector.actionFailed') }}
  </p>

  <div class="inspector__actions">
    <template v-if="confirmingDelete">
      <span class="inspector__confirm">{{ t('timeline.failure.deleteConfirm') }}</span>
      <button type="button" class="dg-button" :disabled="props.pendingAction !== null" @click="confirmingDelete = false">
        {{ t('common.action.cancel') }}
      </button>
      <button
        type="button"
        class="dg-button inspector__delete"
        :disabled="props.pendingAction !== null"
        @click="emit('dismiss', props.failure.batchIds); confirmingDelete = false"
      >
        {{ t('common.action.delete') }}
      </button>
    </template>
    <template v-else>
      <button
        type="button"
        class="dg-button"
        :disabled="!canRetry || props.pendingAction !== null"
        :title="canRetry ? t('timeline.failure.retry') : t('timeline.failure.retryUnavailable')"
        @click="emit('retry', props.failure.batchIds)"
      >
        {{ props.pendingAction === 'retry-batches' ? t('timeline.failure.retrying') : t('common.action.retry') }}
      </button>
      <button
        v-if="canStop"
        type="button"
        class="dg-button"
        :disabled="props.pendingAction !== null"
        :title="t('timeline.failure.stop')"
        @click="emit('stopRetries', props.failure.batchIds)"
      >
        {{ props.pendingAction === 'stop-retries' ? t('timeline.failure.stopping') : t('timeline.failure.stop') }}
      </button>
      <button
        type="button"
        class="dg-button inspector__danger"
        :disabled="!canDelete || props.pendingAction !== null"
        :title="canDelete ? t('timeline.failure.delete') : t('timeline.failure.deleteUnavailable')"
        @click="confirmingDelete = true"
      >
        {{ t('common.action.delete') }}
      </button>
    </template>
    <span v-if="!props.canWrite" class="inspector__readonly">
      {{ t('timeline.inspector.readOnly') }}
    </span>
  </div>
</template>

<style scoped>
.inspector__failure-kind {
  padding: 6px 10px;
  border: 1px solid color-mix(in srgb, var(--dg-danger) 30%, transparent);
  border-radius: 7px;
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
}

.inspector__failure-message {
  padding: 10px 12px;
  border-radius: 7px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  line-height: 1.6;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.failure-batches {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.failure-batches li {
  padding: 3px 8px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 6px;
  background: var(--dg-track-fill);
  color: var(--dg-text-muted);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}

.card-time--failure { color: var(--dg-danger); }
</style>
