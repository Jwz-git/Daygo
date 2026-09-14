<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { useChatStore } from '@/stores/chat'

import ChatComposer from './ChatComposer.vue'
import ChatContextBar from './ChatContextBar.vue'
import ChatDrawer from './ChatDrawer.vue'
import ChatTranscript from './ChatTranscript.vue'
import ChatWelcome from './ChatWelcome.vue'

/*
 * Page owns: drawer toggle, error banner, unavailable state, layout.
 * Transcript / Composer / ContextBar handle their own interactions.
 * Sidebar is now a floating drawer (ChatDrawer), opened on demand.
 */
const { t } = useI18n()
const store = useChatStore()

const actionError = ref('')
const drawerOpen = ref(false)

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

function openDrawer(): void {
  drawerOpen.value = true
}

function closeDrawer(): void {
  drawerOpen.value = false
}

function onDrawerError(message: string): void {
  actionError.value = message
}

onMounted(() => {
  void store.hydrate()
})
</script>

<template>
  <div class="page chat">
    <!-- Error banner -->
    <p v-if="actionError || store.refreshFailed" class="chat-error" role="alert">
      {{ actionError || t('chat.loadError') }}
      <button v-if="store.activeId" type="button" class="dg-button" @click="retrySelect">
        {{ t('chat.retry') }}
      </button>
    </p>

    <!-- Unavailable state -->
    <div v-if="store.unavailable" class="unavailable">
      <h2>{{ t('chat.unavailableTitle') }}</h2>
      <p>{{ t('chat.unavailableDescription') }}</p>
      <button class="dg-button" @click="store.hydrate">{{ t('chat.retry') }}</button>
    </div>

    <!-- Main layout — the glass panel is the entire right column; the drawer
         toggle moves inside the panel head so there's no outer titlebar
         competing with the panel's own frame. -->
    <div v-else class="layout">
      <LiquidGlassSurface intensity="glass" class="panel">
        <!-- Context bar + drawer toggle live at the top of the panel -->
        <div v-if="store.activeConversation !== null" class="panel__head">
          <div class="panel__headRow">
            <h2 class="panel__title">
              {{ store.activeConversation.title || t('chat.newConversation') }}
            </h2>
            <button
              type="button"
              class="header-toggle"
              :aria-label="t('chat.showSidebar')"
              @click="openDrawer"
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none">
                <path d="M3 6h18M3 12h18M3 18h18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
          <ChatContextBar />
        </div>

        <!-- Transcript -->
        <div class="panel__body">
          <ChatTranscript>
            <template #empty>
              <ChatWelcome />
            </template>
          </ChatTranscript>
        </div>

        <!-- Composer -->
        <div v-if="store.activeConversation !== null" class="panel__foot">
          <ChatComposer @error="actionError = $event" />
        </div>
      </LiquidGlassSurface>
    </div>

    <!-- Drawer -->
    <ChatDrawer
      :open="drawerOpen"
      @close="closeDrawer"
      @error="onDrawerError"
    />
  </div>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.chat-error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  padding: 6px var(--dg-page-padding);
  color: var(--dg-danger);
  background: var(--dg-danger-fill);
  font-size: 12px;
}

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
  margin: 0;
}

.unavailable p {
  color: var(--dg-text-secondary);
  font-size: 13px;
  margin: 0;
}

.layout {
  display: flex;
  flex: 1;
  min-height: 0;
  /* The glass panel fills the whole shell panel so its frame reads as the
     window's frame — no inset, otherwise the shell panel's edge peeks out
     behind it like a stacked card. */
  padding: 0;
}

/* ---- glass panel ---- */

.panel {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  /* Match the shell panel's corner so the covered frame never peeks
     through at the corners. */
  border-radius: var(--dg-panel-radius);
  padding: 0 16px;
}

.panel__head {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 0 8px;
  flex: none;
  border-bottom: 1px solid var(--dg-card-border);
}

.panel__headRow {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.panel__title {
  margin: 0;
  flex: 1;
  min-width: 0;
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.panel__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  padding: 0 4px;
}

.panel__foot {
  flex: none;
  padding: 4px 4px 8px;
}

/* ---- header toggle button (in panel head) ---- */
.header-toggle {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  cursor: pointer;
  transition: background var(--dg-motion-base) ease, color var(--dg-motion-base) ease;
}

.header-toggle:hover {
  background: var(--dg-control-fill);
  color: var(--dg-text-primary);
}

.header-toggle:focus-visible {
  outline: 2px solid var(--dg-accent-text);
  outline-offset: 2px;
}

/* ---- narrow viewport ---- */

@media (max-width: 700px) {
  .panel {
    padding: 0 8px;
  }
}
</style>
