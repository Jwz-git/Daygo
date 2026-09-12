<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import { getSettings } from '@/api/settings'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()
const store = useChatStore()

onMounted(() => {
  void store.hydrate()
})

// ---- sidebar views ----

type SidebarView = 'conversations' | 'memory'
const sidebarView = ref<SidebarView>('conversations')

// ---- conversation list ----

const pendingRemoveId = ref<string | null>(null)

async function confirmRemove(): Promise<void> {
  const id = pendingRemoveId.value
  if (id === null) return
  pendingRemoveId.value = null
  await store.removeConversation(id)
}

// ---- transcript ----

const scroller = ref<HTMLElement | null>(null)

async function scrollToBottom(): Promise<void> {
  await nextTick()
  const element = scroller.value
  if (element !== null) element.scrollTop = element.scrollHeight
}

watch(
  () => store.messages.length,
  () => {
    void scrollToBottom()
  },
)
watch(
  () => store.activeId,
  () => {
    void scrollToBottom()
  },
)

function statusLabel(status: string): string {
  if (status === 'failed') return t('chat.status.failed')
  if (status === 'canceled') return t('chat.status.canceled')
  return ''
}

// ---- composer ----

const draft = ref('')

/** A thread without a provider cannot send; the composer is disabled then. */
const providerMissing = ref(false)

watch(
  () => store.activeConversation?.providerId,
  (providerId) => {
    providerMissing.value = providerId === ''
  },
  { immediate: true },
)

async function submit(): Promise<void> {
  const content = draft.value.trim()
  if (content === '' || store.pending || store.activeId === null || providerMissing.value) return
  draft.value = ''
  await store.send(content)
}

function onComposerKeydown(event: KeyboardEvent): void {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    void submit()
  }
}

// ---- global memory ----

const memoryDraft = ref('')
const memorySaved = ref(false)

watch(sidebarView, (view) => {
  if (view === 'memory' && memoryDraft.value === '' && !memoryLoaded) void loadMemory()
})

let memoryLoaded = false

async function loadMemory(): Promise<void> {
  try {
    const settings = await getSettings()
    memoryDraft.value = settings.chat?.memory ?? ''
  } catch {
    // The panel simply starts empty; saving still works once settings load.
  }
  memoryLoaded = true
}

async function saveMemory(): Promise<void> {
  await store.saveMemory(memoryDraft.value)
  memorySaved.value = true
  setTimeout(() => {
    memorySaved.value = false
  }, 2000)
}

// ---- provider select ----

function onProviderChange(event: Event): void {
  const target = event.target as HTMLSelectElement | null
  if (target === null) return
  void store.pinProvider(target.value)
}
</script>

<template>
  <div class="page chat">
    <PageHeader :title="t('chat.title')" />

    <div v-if="store.unavailable" class="unavailable">
      <h2>{{ t('chat.unavailableTitle') }}</h2>
      <p>{{ t('chat.unavailableDescription') }}</p>
    </div>

    <div v-else class="layout">
      <!-- Transcript -->
      <section class="main">
        <template v-if="store.activeConversation !== null">
          <header class="main__head">
            <h2 class="main__title">
              {{ store.activeConversation.title || t('chat.newConversation') }}
            </h2>
            <label class="main__provider">
              <span class="dg-field-label">{{ t('chat.provider.label') }}</span>
              <select
                class="dg-input"
                :value="store.activeConversation.providerId"
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
          </header>

          <div ref="scroller" class="main__messages dg-scroll">
            <p v-if="store.messages.length === 0" class="main__empty">
              {{ t('chat.emptyConversations') }}
            </p>
            <div
              v-for="message in store.messages"
              :key="message.id"
              class="bubble"
              :class="`bubble--${message.role}`"
            >
              <p class="bubble__text">{{ message.content }}</p>
              <span
                v-if="message.role === 'assistant' && message.status !== 'ok' && message.status !== ''"
                class="bubble__status"
                :class="`bubble__status--${message.status}`"
              >
                {{ statusLabel(message.status) }}
              </span>
            </div>
          </div>

          <form class="composer" @submit.prevent="submit">
            <textarea
              v-model="draft"
              class="dg-input composer__input"
              rows="1"
              :placeholder="t('chat.composer.placeholder')"
              :disabled="store.pending || providerMissing"
              @keydown="onComposerKeydown"
            />
            <button
              v-if="!store.pending"
              class="dg-button dg-button--primary"
              type="submit"
              :disabled="draft.trim() === '' || providerMissing"
            >
              {{ t('chat.composer.send') }}
            </button>
            <button
              v-else
              class="dg-button"
              type="button"
              @click="store.cancel"
            >
              {{ t('chat.composer.cancel') }}
            </button>
          </form>
        </template>

        <p v-else class="main__empty">
          {{ t('chat.noSelection') }}
        </p>
      </section>

      <!-- Sidebar, on the right -->
      <aside class="side">
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

        <!-- Conversations view -->
        <div v-if="sidebarView === 'conversations'" class="side__pane">
          <button
            type="button"
            class="dg-button dg-button--primary side__new"
            @click="store.newConversation"
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
                @click="store.select(conversation.id)"
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
            v-model="memoryDraft"
            class="dg-input memory__text"
            rows="14"
            :placeholder="t('chat.memory.placeholder')"
          />
          <div class="memory__row">
            <button type="button" class="dg-button dg-button--primary" @click="saveMemory">
              {{ t('chat.memory.save') }}
            </button>
            <span v-if="memorySaved" class="memory__saved">
              {{ t('chat.memory.saved') }}
            </span>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
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

/* ---- sidebar (right) ---- */

.side {
  display: flex;
  flex-direction: column;
  flex: none;
  width: 260px;
  min-width: 0;
}

.side__tabs {
  display: flex;
  flex: none;
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
  flex-direction: column;
  align-items: flex-end;
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
  resize: vertical;
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

.main__provider {
  display: flex;
  align-items: center;
  gap: 8px;
}

.main__provider .dg-input {
  width: 180px;
}

.main__messages {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  padding: 12px 0;
  overflow-y: auto;
}

.main__empty {
  margin: auto;
  color: var(--dg-text-muted);
  font-size: 13px;
}

/* ---- bubbles ---- */

.bubble {
  max-width: 78%;
  padding: 9px 12px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.55;
}

.bubble--user {
  align-self: flex-end;
  border-bottom-right-radius: 4px;
  background: var(--dg-control-fill);
  color: var(--dg-text-primary);
}

.bubble--assistant {
  align-self: flex-start;
  border: 1px solid var(--dg-card-border);
  border-bottom-left-radius: 4px;
  background: var(--dg-card-fill);
  color: var(--dg-text-primary);
}

.bubble__text {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.bubble__status {
  display: inline-block;
  margin-top: 6px;
  padding: 2px 7px;
  border-radius: 999px;
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
  font-size: 10px;
  font-weight: 620;
}

/* ---- composer ---- */

.composer {
  display: flex;
  flex: none;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--dg-card-border);
}

.composer__input {
  flex: 1;
  min-height: 36px;
  resize: none;
  font-size: 13px;
}

/* ---- narrow ---- */

@media (max-width: 640px) {
  .layout {
    flex-direction: column;
  }

  .side {
    width: 100%;
  }

  .side__list {
    max-height: 160px;
  }

  .memory__text {
    min-height: 96px;
  }
}
</style>
