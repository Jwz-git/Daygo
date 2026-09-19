<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { useChatStore } from '@/stores/chat'

const { t } = useI18n()
const store = useChatStore()

const emit = defineEmits<{
  error: [message: string]
}>()

async function perform(action: () => Promise<unknown>): Promise<void> {
  emit('error', '')
  try {
    await action()
  } catch {
    emit('error', t('chat.actionError'))
  }
}

// ---- draft ----

const inputEl = ref<HTMLTextAreaElement | null>(null)
const drafts = ref<Record<string, string>>({})
const draft = computed({
  get: () => drafts.value[store.activeId ?? ''] ?? '',
  set: (value: string) => { drafts.value[store.activeId ?? ''] = value },
})

// A welcome-screen suggestion sends immediately. If it can't send yet (no
// provider, a turn in flight), the text stays in the composer and takes focus
// so the user can resolve the blocker and submit manually.
async function sendPrompt(text: string): Promise<void> {
  draft.value = text
  if (!canSend.value) {
    void nextTick(() => inputEl.value?.focus())
    return
  }
  await submit()
}

defineExpose({ sendPrompt })

const providerMissing = computed(() => !store.activeConversation?.providerId)
const tooLong = computed(() => new TextEncoder().encode(draft.value.trim()).length > 32 * 1024)
const canSend = computed(() =>
  draft.value.trim() !== '' &&
  !store.pending &&
  !store.loading &&
  !providerMissing.value &&
  !tooLong.value &&
  !store.refreshFailed,
)

async function submit(): Promise<void> {
  const content = draft.value.trim()
  const id = store.activeId
  if (!canSend.value || id === null) return
  await perform(async () => {
    if (await store.send(content)) drafts.value[id] = ''
  })
}

function onComposerKeydown(event: KeyboardEvent): void {
  if (event.isComposing || event.keyCode === 229) return
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    void submit()
  }
}

async function cancel(): Promise<void> {
  await perform(() => store.cancel())
}

// ---- provider & model (inline in context bar now, just for working indicator) ----

const effectiveModel = computed(() => {
  const conv = store.activeConversation
  if (conv === null) return ''
  const provider = store.providers.find((p) => p.id === conv.providerId)
  return conv.model !== '' ? conv.model : provider?.model ?? ''
})
</script>

<template>
  <!-- Working / pending indicator -->
  <p v-if="store.pending" class="working" role="status">
    <span class="working__spinner" aria-hidden="true" />
    {{ t('chat.working') }}
    <span v-if="effectiveModel !== ''" class="working__model">{{ effectiveModel }}</span>
  </p>

  <p v-if="tooLong" class="composer-error" role="alert">{{ t('chat.tooLong') }}</p>

  <!-- Input area -->
  <form class="composer" @submit.prevent="submit">
    <textarea
      ref="inputEl"
      v-model="draft"
      class="dg-input composer__input"
      rows="1"
      :placeholder="t('chat.composer.placeholder')"
      :aria-label="t('chat.composer.placeholder')"
      :disabled="store.pending || store.loading || providerMissing"
      @keydown="onComposerKeydown"
    />
    <button
      v-if="!store.pending"
      class="dg-button dg-button--primary composer__send"
      type="submit"
      :disabled="!canSend"
      :aria-label="t('chat.composer.send')"
    >
      <!-- Paper plane icon -->
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
        <path d="M14 2L2 7l5 2 2 5 5-12z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </button>
    <button
      v-else
      class="dg-button composer__cancel"
      type="button"
      :aria-label="t('chat.composer.cancel')"
      @click="cancel"
    >
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
        <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
    </button>
  </form>
</template>

<style scoped>
.working {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 6px;
  color: var(--dg-text-muted);
  font-size: 11px;
}

.working__spinner {
  flex: none;
  width: 10px;
  height: 10px;
  border: 2px solid var(--dg-chip-border);
  border-top-color: var(--dg-accent-text);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.working__model {
  color: var(--dg-text-muted);
  font-family: var(--dg-font-mono);
  font-size: 10px;
}

.composer-error {
  color: var(--dg-danger);
  font-size: 12px;
  margin: 0 0 6px;
}

/* ---- composer ---- */

.composer {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  flex: none;
  padding: 8px 0 0;
  border-top: 1px solid var(--dg-card-border);
}

.composer__input {
  flex: 1;
  min-height: 36px;
  max-height: 140px;
  resize: none;
  font-size: 13px;
  line-height: 1.5;
  padding: 8px 12px;
}

.composer__send {
  flex: none;
  width: 36px;
  height: 36px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
}

.composer__send:disabled {
  opacity: 0.4;
}

.composer__cancel {
  flex: none;
  width: 36px;
  height: 36px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  border: 1px solid var(--dg-chip-border);
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
}

.composer__cancel:hover {
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
  border-color: var(--dg-danger);
}
</style>
