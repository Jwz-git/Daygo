<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ChatConversationDTO } from '@/api/dto'
import DgSelect, { type DgSelectOption } from '@/components/DgSelect.vue'
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

const providerOptions = computed<DgSelectOption[]>(() => [
  { value: '', label: t('chat.provider.placeholder') },
  ...store.providers.map((provider) => ({ value: provider.id, label: provider.displayName })),
])

const modelOptions = computed<DgSelectOption[]>(() => {
  const options: DgSelectOption[] = [{ value: '', label: t('chat.model.follow', { model: activeProvider.value?.model ?? '' }) }]
  if (activeProvider.value?.model) options.push({ value: activeProvider.value.model, label: activeProvider.value.model })
  return options
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
      <DgSelect
        class="context-bar__select"
        size="sm"
        :model-value="activeConversation?.providerId ?? ''"
        :options="providerOptions"
        :aria-label="t('chat.provider.label')"
        :disabled="store.pending || store.loading"
        @update:model-value="switchProvider"
      />
    </div>

    <!-- Model (only when provider is set) -->
    <div v-if="!providerMissing" class="context-bar__group">
      <span class="context-bar__label">{{ t('chat.model.label') }}</span>
      <DgSelect
        class="context-bar__select"
        size="sm"
        :model-value="activeConversation?.model ?? ''"
        :options="modelOptions"
        :aria-label="t('chat.model.label')"
        :disabled="store.pending || store.loading"
        @update:model-value="switchModel"
      />
    </div>
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
  min-width: 140px;
}

</style>
