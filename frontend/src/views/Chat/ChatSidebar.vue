<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { getSettings } from '@/api/settings'
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

// ---- sidebar views ----

type SidebarView = 'conversations' | 'memory'
const sidebarView = ref<SidebarView>('conversations')
const sidebarVisible = ref(true)

// ---- conversation list ----

const pendingRemoveId = ref<string | null>(null)

async function confirmRemove(): Promise<void> {
  const id = pendingRemoveId.value
  if (id === null) return
  pendingRemoveId.value = null
  await perform(() => store.removeConversation(id))
}

// ---- global memory ----

const memoryDraft = ref('')
const memorySaved = ref(false)
const memoryInput = ref<HTMLTextAreaElement | null>(null)

/** The textarea grows with its content; the user cannot drag-resize it. */
function autoGrow(element: HTMLTextAreaElement | null): void {
  if (element === null) return
  element.style.height = 'auto'
  element.style.height = `${element.scrollHeight}px`
}

watch(memoryDraft, async () => {
  await nextTick()
  autoGrow(memoryInput.value)
})

watch(sidebarView, (view) => {
  if (view === 'memory' && !memoryLoaded.value) void loadMemory()
})

const memoryLoaded = ref(false)
const memorySaving = ref(false)
let savedTimer: ReturnType<typeof setTimeout> | undefined
onUnmounted(() => clearTimeout(savedTimer))
watch(memoryDraft, () => {
  memorySaved.value = false
})

async function loadMemory(): Promise<void> {
  try {
    const settings = await getSettings()
    memoryDraft.value = settings.chat?.memory ?? ''
    memoryLoaded.value = true
    await nextTick()
    autoGrow(memoryInput.value)
  } catch {
    emit('error', t('chat.loadError'))
  }
}

async function saveMemory(): Promise<void> {
  if (!memoryLoaded.value || memorySaving.value) return
  memorySaving.value = true
  const value = memoryDraft.value
  await perform(async () => {
    await store.saveMemory(value)
    memorySaved.value = memoryDraft.value === value
    clearTimeout(savedTimer)
    savedTimer = setTimeout(() => { memorySaved.value = false }, 2000)
  })
  memorySaving.value = false
}
</script>

<template>
  <button
    v-if="!sidebarVisible"
    type="button"
    class="side__toggle side__toggle--show"
    :aria-label="t('chat.showSidebar')"
    @click="sidebarVisible = true"
  >
    ‹
  </button>

  <!-- Conversation navigation -->
  <aside v-else class="side">
    <div class="side__header">
      <nav class="side__tabs" :aria-label="t('chat.title')">
      <button
        type="button"
        class="side__tab"
        :class="{ 'side__tab--active': sidebarView === 'conversations' }"
        @click="sidebarView = 'conversations'"
      >
        {{ t('chat.conversations') }}
      </button>
      <button
        type="button"
        class="side__tab"
        :class="{ 'side__tab--active': sidebarView === 'memory' }"
        @click="sidebarView = 'memory'"
      >
        {{ t('chat.memory.title') }}
      </button>
      </nav>
      <button
        type="button"
        class="side__toggle"
        :aria-label="t('chat.hideSidebar')"
        @click="sidebarVisible = false"
      >
        ›
      </button>
    </div>

    <!-- Conversations view -->
    <div v-if="sidebarView === 'conversations'" class="side__pane">
      <button
        type="button"
        class="dg-button dg-button--primary side__new"
        @click="perform(store.newConversation)"
      >
        {{ t('chat.newConversation') }}
      </button>

      <ul class="side__list dg-scroll" :aria-label="t('chat.conversations')">
        <li v-if="store.listedConversations.length === 0" class="side__empty">
          {{ t('chat.emptyConversations') }}
        </li>
        <li
          v-for="conversation in store.listedConversations"
          :key="conversation.id"
          class="side__item"
          :class="{ 'side__item--active': conversation.id === store.activeId }"
        >
          <button
            type="button"
            class="side__open"
            @click="perform(() => store.select(conversation.id))"
          >
            <span class="side__title">{{ conversation.title || t('chat.newConversation') }}</span>
          </button>
          <div v-if="pendingRemoveId === conversation.id" class="side__confirm">
            <span>{{ t('chat.removeConfirm') }}</span>
            <button type="button" class="dg-button" @click="pendingRemoveId = null">
              {{ t('common.action.cancel') }}
            </button>
            <button
              type="button"
              class="dg-button dg-button--primary"
              @click="confirmRemove"
            >
              {{ t('common.action.delete') }}
            </button>
          </div>
          <button
            v-else
            type="button"
            class="side__delete"
            :aria-label="t('chat.deleteConversation')"
            @click="pendingRemoveId = conversation.id"
          >
            ×
          </button>
        </li>
      </ul>
    </div>

    <!-- Global memory view -->
    <div v-else class="side__pane memory">
      <p class="memory__hint">{{ t('chat.memory.hint') }}</p>
      <textarea
        ref="memoryInput"
        v-model="memoryDraft"
        :disabled="!memoryLoaded"
        :aria-label="t('chat.memory.title')"
        class="dg-input memory__text"
        rows="6"
        :placeholder="t('chat.memory.placeholder')"
        @input="autoGrow(memoryInput)"
      />
      <button v-if="!memoryLoaded" class="dg-button" @click="loadMemory">{{ t('chat.retry') }}</button>
      <div class="memory__row">
        <button type="button" class="dg-button dg-button--primary" :disabled="!memoryLoaded || memorySaving" @click="saveMemory">
          {{ t('chat.memory.save') }}
        </button>
        <span v-if="memorySaved" class="memory__saved">
          {{ t('chat.memory.saved') }}
        </span>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.side {
  display: flex;
  flex-direction: column;
  flex: none;
  width: 260px;
  min-width: 0;
}

.side__header {
  display: flex;
  align-items: stretch;
  gap: 4px;
}

.side__toggle {
  flex: none;
  width: 32px;
  min-height: 32px;
  padding: 0;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
}

.side__toggle:hover {
  background: var(--dg-control-fill);
  color: var(--dg-text-primary);
}

.side__toggle:focus-visible {
  outline: 2px solid var(--dg-accent-text);
  outline-offset: 2px;
}

.side__toggle--show {
  align-self: flex-start;
}

.side__tabs {
  display: flex;
  flex: 1;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.side__tab {
  flex: 1;
  padding: 5px 8px;
  border: none;
  border-radius: 5px;
  background: none;
  color: var(--dg-text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: background-color var(--dg-motion-base) ease, color var(--dg-motion-base) ease;
}

.side__tab--active {
  background: var(--dg-control-fill);
  color: var(--dg-text-primary);
  font-weight: 600;
}

.side__pane {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
}

.side__new {
  flex: none;
}

.side__list {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
  min-height: 0;
  margin: 0;
  padding: 0;
  list-style: none;
  overflow-y: auto;
}

.side__empty {
  padding: 10px;
  color: var(--dg-text-muted);
  font-size: 12px;
}

.side__item {
  display: flex;
  align-items: center;
  gap: 4px;
  border-radius: 6px;
  transition: background-color var(--dg-motion-base) ease;
}

.side__item:hover,
.side__item--active {
  background: var(--dg-control-fill-hover);
}

.side__open {
  flex: 1;
  min-width: 0;
  padding: 8px 8px;
  border: none;
  background: none;
  text-align: left;
  cursor: pointer;
}

.side__title {
  display: block;
  color: var(--dg-text-primary);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.side__delete,
.side__confirm {
  flex: none;
}

.side__delete {
  padding: 4px 8px;
  margin-right: 4px;
  border: none;
  border-radius: 4px;
  background: none;
  color: var(--dg-text-muted);
  font-size: 14px;
  cursor: pointer;
}

.side__delete:hover {
  color: var(--dg-danger);
}

.side__confirm {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 4px;
  padding: 6px 8px;
}

.side__confirm span {
  color: var(--dg-text-primary);
  font-size: 11px;
}

.side__confirm .dg-button {
  padding: 3px 8px;
  font-size: 11px;
}

/* ---- global memory view ---- */

.memory {
  padding-top: 2px;
}

.memory__hint {
  margin: 0;
  color: var(--dg-text-muted);
  font-size: 11px;
}

.memory__text {
  flex: 1;
  width: 100%;
  min-height: 120px;
  max-height: 100%;
  resize: none;
  overflow-y: auto;
  font-size: 12px;
}

.memory__row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.memory__row .dg-button {
  padding: 4px 10px;
  font-size: 11px;
}

.memory__saved {
  color: var(--dg-accent-text);
  font-size: 11px;
}

/* ---- narrow ---- */

@media (max-width: 900px) {
  .side {
    width: 100%;
  }

  .side__list {
    max-height: 120px;
  }

  .memory__text {
    min-height: 96px;
  }
}
</style>
