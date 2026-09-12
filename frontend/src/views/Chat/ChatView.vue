<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import PlannedNotice from '@/components/PlannedNotice.vue'

const { t } = useI18n()
</script>

<template>
  <div class="page chat">
    <PageHeader :title="t('chat.title')">
      <template #lead><span class="dg-chip dg-chip--filled">{{ t('common.placeholder.planned') }}</span></template>
    </PageHeader>

    <div class="content dg-scroll">
      <PlannedNotice
        title-key="chat.plannedTitle"
        description-key="chat.plannedDescription"
      />
    </div>

    <form class="composer" @submit.prevent>
      <input
        class="dg-input composer__input"
        type="text"
        :placeholder="t('chat.composer.placeholder')"
        :aria-label="t('chat.composer.placeholder')"
        disabled
      >
      <button class="dg-button dg-button--primary" type="submit" disabled>
        {{ t('chat.composer.send') }}
      </button>
    </form>
  </div>
</template>

<style scoped>
/*
 * Placeholder only: no store, no bindings, no events. Everything is inert
 * until the chat service and its bindings exist (docs/modules/chat.md slice 5).
 */
.content {
  display: flex;
  flex-direction: column;
  justify-content: center;
  flex: 1;
  min-height: 0;
  padding: 0 var(--dg-page-padding) 14px;
}

.composer {
  display: flex;
  gap: 10px;
  padding: 14px var(--dg-page-padding) var(--dg-page-padding);
  border-top: 1px solid var(--dg-card-border);
}

.composer__input {
  flex: 1;
}

@media (max-width: 560px) {
  .composer {
    flex-direction: column;
  }
}
</style>
