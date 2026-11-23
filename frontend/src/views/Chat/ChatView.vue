<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { useChatStore } from '@/stores/chat'

import ChatComposer from './ChatComposer.vue'
import ChatContextBar from './ChatContextBar.vue'
import ChatDrawer from './ChatDrawer.vue'
import ChatTranscript from './ChatTranscript.vue'
import ChatWelcome from './ChatWelcome.vue'
import ConversationTitle from './ConversationTitle.vue'

type ComposerExposed = { sendPrompt: (text: string) => void }

/*
 * Page owns: drawer toggle, error banner, unavailable state, layout.
 * Transcript / Composer / ContextBar handle their own interactions.
 * Sidebar is now a floating drawer (ChatDrawer), opened on demand.
 */
const { t } = useI18n()
const store = useChatStore()

const actionError = ref('')
const drawerOpen = ref(false)
const renameError = ref('')
const renaming = ref(false)
const composer = ref<ComposerExposed | null>(null)

function onHintSelect(prompt: string): void {
  composer.value?.sendPrompt(prompt)
}

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

async function onRename(committed: string): Promise<void> {
  const id = store.activeId
  if (id === null) return
  renaming.value = true
  renameError.value = ''
  try {
    await store.renameConversation(id, committed)
  } catch {
    renameError.value = t('chat.actionError')
  } finally {
    renaming.value = false
  }
}

onMounted(() => {
  void store.hydrate()
})
onBeforeUnmount(() => store.stopListening())
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

    <!-- Main layout — sits directly inside AppShell's .dg-panel so there is
         no nested glass frame competing with the shell's own border. -->
    <div v-else class="layout">
      <div class="chat-surface">
        <!-- Context bar + drawer toggle live at the top of the panel -->
        <div v-if="store.activeConversation !== null" class="panel__head">
          <div class="panel__headRow">
            <ConversationTitle
              class="panel__title-wrap"
              :title="store.activeConversation.title"
              :disabled="store.pending || store.loading || store.refreshFailed"
              :busy="renaming"
              :fallback-title="t('chat.newConversation')"
              :placeholder="t('chat.renameTitlePlaceholder')"
              :empty-message="t('chat.renameTitleRequired')"
              :error-text="renameError"
              @commit="onRename"
            />
            <button
              type="button"
              class="header-toggle"
              :aria-label="t('chat.showSidebar')"
              @click="openDrawer"
            >
              <DgIcon name="menu" :size="18" />
            </button>
          </div>
          <ChatContextBar />
        </div>

        <!-- Transcript -->
        <div class="panel__body">
          <ChatTranscript>
            <template #empty>
              <ChatWelcome @select="onHintSelect" />
            </template>
          </ChatTranscript>
        </div>

        <!-- Composer -->
        <div v-if="store.activeConversation !== null" class="panel__foot">
          <ChatComposer ref="composer" @error="actionError = $event" />
        </div>
      </div>
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

/* ---- chat surface ---- */

.chat-surface {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.panel__head {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 18px var(--dg-page-padding) 12px;
  flex: none;
  border-bottom: 1px solid var(--dg-card-border);
  -webkit-app-region: drag;
  --wails-draggable: drag;
}

.panel__headRow {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.panel__title-wrap {
  flex: 1;
  min-width: 0;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}

.panel__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

.panel__foot {
  flex: none;
  padding: 6px var(--dg-page-padding) 16px;
}

/* ---- header toggle button (in panel head) ---- */
.header-toggle {
  width: 32px;
  height: 32px;
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--dg-text-secondary);
  cursor: pointer;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
  transition: background var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.header-toggle:hover {
  background: var(--dg-hover-fill-strong);
  color: var(--dg-text-primary);
}

.header-toggle:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

/* ---- narrow viewport ---- */

@media (max-width: 700px) {
  .panel__head {
    padding: 14px 16px 10px;
  }

  .panel__foot {
    padding: 6px 16px 12px;
  }
}
</style>
