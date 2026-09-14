<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import ComboBox from '@/components/ComboBox.vue'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()
const store = useChatStore()

/*
 * Failures surface in the page-level banner, not locally: the emit carries
 * the message and an empty string clears it when the next action starts.
 */
const emit = defineEmits<{ error: [message: string] }>()

async function perform(action: () => Promise<unknown>): Promise<void> {
  emit('error', '')
  try {
    await action()
  } catch {
    emit('error', t('chat.actionError'))
  }
}

// ---- draft ----

const drafts = ref<Record<string, string>>({})
const draft = computed({
  get: () => drafts.value[store.activeId ?? ''] ?? '',
  set: (value: string) => { drafts.value[store.activeId ?? ''] = value },
})

/** A thread without a provider cannot send; the composer is disabled then. */
const providerMissing = computed(() => !store.activeConversation?.providerId)
const tooLong = computed(() => new TextEncoder().encode(draft.value.trim()).length > 32 * 1024)

async function submit(): Promise<void> {
  const content = draft.value.trim()
  const id = store.activeId
  if (content === '' || store.pending || store.loading || id === null || providerMissing.value || tooLong.value) return
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

// ---- provider & model select ----

function onProviderChange(event: Event): void {
  const target = event.target as HTMLSelectElement | null
  if (target === null) return
  void perform(() => store.pinProvider(target.value))
}

/** The pinned provider's row, for its configured model. */
const activeProvider = computed(() =>
  store.providers.find((provider) => provider.id === store.activeConversation?.providerId) ?? null,
)

/** The model the thread will actually use: the override, else the provider's. */
const effectiveModel = computed(() => {
  const conversation = store.activeConversation
  if (conversation === null) return ''
  return conversation.model !== '' ? conversation.model : activeProvider.value?.model ?? ''
})

const modelOptions = computed(() => {
  const provider = activeProvider.value
  if (provider === null) return []
  return [
    { value: '', label: t('chat.model.follow', { model: provider.model }) },
    { value: provider.model, label: provider.model },
  ]
})

function onModelChange(model: string): void {
  void perform(() => store.pinModel(model))
}
</script>

<template>
  <p v-if="store.pending" class="working" role="status">
    <span class="working__spinner" aria-hidden="true" />
    {{ t('chat.working') }}
    <span v-if="effectiveModel !== ''" class="working__model">{{ effectiveModel }}</span>
  </p>
  <p v-if="tooLong" class="composer-error" role="alert">{{ t('chat.tooLong') }}</p>
  <div class="composer-bar">
    <label class="composer-bar__field">
      <span class="dg-field-label">{{ t('chat.provider.label') }}</span>
      <select
        class="dg-input"
        :value="store.activeConversation?.providerId ?? ''"
        :disabled="store.pending || store.loading"
        @change="onProviderChange"
      >
        <option v-if="providerMissing" value="">
          {{ t('chat.provider.placeholder') }}
        </option>
        <option
          v-for="provider in store.providers"
          :key="provider.id"
          :value="provider.id"
        >
          {{ provider.displayName }}
        </option>
      </select>
    </label>
    <label v-if="!providerMissing" class="composer-bar__field">
      <span class="dg-field-label">{{ t('chat.model.label') }}</span>
      <ComboBox
        :model-value="store.activeConversation?.model ?? ''"
        :options="modelOptions"
        :fallback-label="t('chat.model.custom')"
        :placeholder="activeProvider?.model ?? ''"
        :aria-label="t('chat.model.label')"
        :disabled="store.pending || store.loading"
        @update:model-value="onModelChange"
      />
    </label>
  </div>
  <form class="composer" @submit.prevent="submit">
    <textarea
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
      class="dg-button dg-button--primary"
      type="submit"
      :disabled="draft.trim() === '' || providerMissing || store.loading || store.refreshFailed || tooLong"
    >
      {{ t('chat.composer.send') }}
    </button>
    <button
      v-else
      class="dg-button"
      type="button"
      @click="perform(store.cancel)"
    >
      {{ t('chat.composer.cancel') }}
    </button>
  </form>
</template>

<style scoped>
.working {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  padding: 4px 0;
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
  animation: working-spin 0.8s linear infinite;
}

@keyframes working-spin {
  to {
    transform: rotate(360deg);
  }
}

.working__model {
  color: var(--dg-text-muted);
  font-family: var(--dg-font-mono);
}

.composer-error {
  color: var(--dg-danger);
  font-size: 12px;
}

/* ---- composer bar (provider & model, above the input) ---- */

.composer-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 10px;
  flex: none;
  padding-top: 10px;
  border-top: 1px solid var(--dg-card-border);
}

.composer-bar__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.composer-bar__field .dg-input,
.composer-bar__field .combo {
  width: 190px;
}

/* ---- composer ---- */

.composer {
  display: flex;
  flex: none;
  gap: 10px;
  padding-top: 10px;
}

.composer__input {
  flex: 1;
  min-height: 36px;
  resize: none;
  font-size: 13px;
}
</style>
