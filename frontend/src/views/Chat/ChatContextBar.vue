<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ChatConversationDTO } from '@/api/dto'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()
const store = useChatStore()

const activeConversation = computed(() => store.activeConversation)

const activeProvider = computed(() =>
  store.providers.find((p) => p.id === activeConversation.value?.providerId) ?? null,
)

const effectiveModel = computed(() => {
  const conv = activeConversation.value
  if (conv === null) return ''
  return conv.model !== '' ? conv.model : activeProvider.value?.model ?? ''
})

async function switchProvider(id: string): Promise<void> {
  await store.pinProvider(id)
}

async function switchModel(model: string): Promise<void> {
  await store.pinModel(model)
}

const providerMissing = computed(() => !activeConversation.value?.providerId)
</script>

<template>
  <div class="context-bar">
    <!-- Provider -->
    <div class="context-bar__group">
      <span class="context-bar__label">{{ t('chat.provider.label') }}</span>
      <select
        class="context-bar__select"
        :value="activeConversation?.providerId ?? ''"
        :disabled="store.pending || store.loading"
        @change="(e) => switchProvider((e.target as HTMLSelectElement).value)"
      >
        <option value="">
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
    </div>

    <!-- Model (only when provider is set) -->
    <div v-if="!providerMissing" class="context-bar__group">
      <span class="context-bar__label">{{ t('chat.model.label') }}</span>
      <select
        class="context-bar__select"
        :value="activeConversation?.model ?? ''"
        :disabled="store.pending || store.loading"
        @change="(e) => switchModel((e.target as HTMLSelectElement).value)"
      >
        <option value="">
          {{ t('chat.model.follow', { model: activeProvider?.model ?? '' }) }}
        </option>
        <option
          v-if="activeProvider?.model"
          :value="activeProvider.model"
        >
          {{ activeProvider.model }}
        </option>
      </select>
    </div>

    <!-- Edit mode badge -->
    <span class="context-bar__badge context-bar__badge--readonly">
      {{ t('chat.contextBar.readonly') }}
    </span>
  </div>
</template>

<style scoped>
.context-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  flex: none;
  padding: 6px 0;
  border-bottom: 1px solid var(--dg-card-border);
}

.context-bar__group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.context-bar__label {
  color: var(--dg-text-muted);
  font-size: 11px;
  font-weight: 600;
}

.context-bar__select {
  padding: 3px 24px 3px 8px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 6px;
  background: var(--dg-track-fill);
  color: var(--dg-text-primary);
  font-size: 12px;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6' viewBox='0 0 10 6'%3E%3Cpath d='M1 1l4 4 4-4' stroke='%23888' stroke-width='1.5' fill='none' stroke-linecap='round'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 8px center;
  cursor: pointer;
  min-width: 120px;
}

.context-bar__select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.context-bar__badge {
  margin-left: auto;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 620;
}

.context-bar__badge--readonly {
  background: var(--dg-track-fill);
  border: 1px solid var(--dg-chip-border);
  color: var(--dg-text-muted);
}

.context-bar__badge--editable {
  background: color-mix(in srgb, var(--dg-accent) 12%, transparent);
  color: var(--dg-accent-text);
}
</style>
