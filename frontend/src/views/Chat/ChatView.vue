<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import { useChatStore } from '@/stores/chat'

import ChatComposer from './ChatComposer.vue'
import ChatSidebar from './ChatSidebar.vue'
import ChatTranscript from './ChatTranscript.vue'

/*
 * The page owns only orchestration: the error banner, the unavailable state
 * and the two-column layout. Transcript rendering, the composer and the
 * sidebar are separate components; their action failures bubble up through
 * an `error` emit (an empty message clears the banner when an action starts).
 */
const { t } = useI18n()
const store = useChatStore()
const actionError = ref('')

async function retrySelect(): Promise<void> {
  actionError.value = ''
  const id = store.activeId
  if (id === null) return
  try {
    await store.select(id)
  } catch {
    actionError.value = t('chat.actionError')
  }
}

onMounted(() => {
  void store.hydrate()
})
</script>

<template>
  <div class="page chat">
    <PageHeader :title="t('chat.title')" />
    <p v-if="actionError || store.refreshFailed" class="chat-error" role="alert">
      {{ actionError || t('chat.loadError') }}
      <button v-if="store.activeId" type="button" class="dg-button" @click="retrySelect">{{ t('chat.retry') }}</button>
    </p>

    <div v-if="store.unavailable" class="unavailable">
      <h2>{{ t('chat.unavailableTitle') }}</h2>
      <p>{{ t('chat.unavailableDescription') }}</p>
      <button class="dg-button" @click="store.hydrate">{{ t('chat.retry') }}</button>
    </div>

    <div v-else class="layout">
      <!-- Transcript -->
      <section class="main">
        <template v-if="store.activeConversation !== null">
          <header class="main__head">
            <h2 class="main__title">
              {{ store.activeConversation.title || t('chat.newConversation') }}
            </h2>
          </header>

          <ChatTranscript />
          <ChatComposer @error="actionError = $event" />
        </template>

        <p v-else class="main__empty">
          {{ t('chat.noSelection') }}
        </p>
      </section>

      <ChatSidebar @error="actionError = $event" />
    </div>
  </div>
</template>

<style scoped>
.chat-error { color: var(--dg-danger); padding: 0 var(--dg-page-padding); font-size: 12px; }

.unavailable {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 0 var(--dg-page-padding);
}

.unavailable h2 {
  color: var(--dg-text-primary);
  font-size: 15px;
}

.unavailable p {
  color: var(--dg-text-secondary);
  font-size: 13px;
}

.layout {
  display: flex;
  flex: 1;
  gap: 12px;
  min-height: 0;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

/* ---- main column ---- */

.main {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.main__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex: none;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--dg-card-border);
}

.main__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.main__empty {
  margin: auto;
  color: var(--dg-text-muted);
  font-size: 13px;
}

/* ---- narrow ---- */

@media (max-width: 900px) {
  .layout {
    flex-direction: column;
  }
}
</style>
