<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { computed, nextTick, ref, watch } from 'vue'
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

function autoGrow(): void {
  const el = inputEl.value
  if (el === null) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(140, Math.max(38, el.scrollHeight))}px`
}

watch(draft, () => { void nextTick(autoGrow) }, { immediate: true })

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
  <div class="composer-wrap">
    <!-- Working / pending indicator -->
    <p v-if="store.pending" class="working" role="status">
      <span class="working__dots" aria-hidden="true"><i></i><i></i><i></i></span>
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
        @input="autoGrow"
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
        <DgIcon name="send" :size="16" />
      </button>
      <button
        v-else
        class="dg-button composer__cancel"
        type="button"
        :aria-label="t('chat.composer.cancel')"
        @click="cancel"
      >
        <DgIcon name="close" :size="14" />
      </button>
    </form>
  </div>
</template>

<style scoped>
.composer-wrap {
  width: 100%;
  max-width: 780px;
  margin: 0 auto;
}

.working {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 6px;
  color: var(--dg-text-muted);
  font-size: 11.5px;
}

.working__dots {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

/* Three dots that swell and lift in sequence — a "thinking" cadence that reads
   as considered, where a spinning ring reads as a generic wait. */
.working__dots i {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--dg-accent-text);
  opacity: 0.3;
  animation: working-pulse 1.25s ease-in-out infinite;
}

.working__dots i:nth-child(2) {
  animation-delay: 0.16s;
}

.working__dots i:nth-child(3) {
  animation-delay: 0.32s;
}

@keyframes working-pulse {
  0%, 70%, 100% {
    opacity: 0.3;
    transform: translateY(0);
  }
  35% {
    opacity: 1;
    transform: translateY(-2px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .working__dots i {
    animation: none;
    opacity: 0.6;
  }
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
  padding: 10px 0 0;
  border-top: 1px solid var(--dg-card-border);
}

.composer__input {
  flex: 1;
  min-height: 38px;
  max-height: 140px;
  resize: none;
  overflow-y: auto;
  font-size: 13.5px;
  line-height: 1.5;
  padding: 8px 12px;
  border-radius: 10px;
}

.composer__send {
  flex: none;
  width: 38px;
  height: 38px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
}

.composer__send:disabled {
  opacity: 0.4;
}

.composer__cancel {
  flex: none;
  width: 38px;
  height: 38px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
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
