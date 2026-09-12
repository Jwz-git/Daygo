<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import ComboBox from '@/components/ComboBox.vue'
import PageHeader from '@/components/PageHeader.vue'
import { getSettings } from '@/api/settings'
import type { ChatMessageDTO } from '@/api/dto'
import { useChatStore } from '@/stores/chat'

const { t, te } = useI18n()
const store = useChatStore()
const actionError = ref('')
async function perform(action: () => Promise<unknown>): Promise<void> {
  actionError.value = ''
  try {
    await action()
  } catch {
    actionError.value = t('chat.actionError')
  }
}

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
  await perform(() => store.removeConversation(id))
}

// ---- transcript ----

const scroller = ref<HTMLElement | null>(null)

async function scrollToBottom(): Promise<void> {
  await nextTick()
  const element = scroller.value
  if (element !== null) element.scrollTop = element.scrollHeight
}

watch(
  () => store.messages.at(-1)?.id,
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

// ---- tool message groups ----

/**
 * Render units for the transcript. A tool_call and its adjacent tool_result
 * collapse into one group; everything else renders as a plain bubble. A
 * result whose call fell outside the page still renders (with no arguments),
 * so paging can never hide a half-pair.
 */
type RenderItem =
  | { kind: 'message'; message: ChatMessageDTO }
  | { kind: 'tool'; call: ChatMessageDTO | null; result: ChatMessageDTO | null }

const renderItems = computed<RenderItem[]>(() => {
  const items: RenderItem[] = []
  const messages = store.messages
  for (let i = 0; i < messages.length; i++) {
    const message = messages[i]
    if (message.role === 'tool_call') {
      const next = messages[i + 1]
      const result = next !== undefined && next.role === 'tool_result' ? next : null
      if (result !== null) i++
      items.push({ kind: 'tool', call: message, result })
    } else if (message.role === 'tool_result') {
      items.push({ kind: 'tool', call: null, result: message })
    } else {
      items.push({ kind: 'message', message })
    }
  }
  return items
})

const expandedToolIds = ref(new Set<number>())

function groupKey(item: Extract<RenderItem, { kind: 'tool' }>): number {
  return (item.call ?? item.result)?.id ?? 0
}

function isToolExpanded(item: Extract<RenderItem, { kind: 'tool' }>): boolean {
  return expandedToolIds.value.has(groupKey(item))
}

function toggleTool(item: Extract<RenderItem, { kind: 'tool' }>): void {
  const key = groupKey(item)
  const next = new Set(expandedToolIds.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedToolIds.value = next
}

/** Arguments JSON narrowed through unknown, per the Wails boundary rule. */
function parseToolArguments(raw: string): Record<string, unknown> {
  if (raw === '') return {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    // Malformed arguments render as empty; the raw text stays one click away.
  }
  return {}
}

function stringArg(args: Record<string, unknown>, key: string): string {
  const value = args[key]
  return typeof value === 'string' ? value : ''
}

function numberArg(args: Record<string, unknown>, key: string): number | null {
  const value = args[key]
  return typeof value === 'number' ? value : null
}

function toolLabel(item: Extract<RenderItem, { kind: 'tool' }>): string {
  const name = (item.call ?? item.result)?.toolName ?? ''
  const key = `chat.tools.names.${name}`
  if (te(key)) return t(key)
  return name === '' ? t('chat.tools.unknownTool') : `${t('chat.tools.unknownTool')} ${name}`
}

/** One identifying argument per tool, shown on the collapsed line. */
function toolDetail(item: Extract<RenderItem, { kind: 'tool' }>): string {
  const message = item.call ?? item.result
  if (message === null) return ''
  const args = parseToolArguments(item.call?.toolArguments ?? '')
  switch (message.toolName) {
    case 'timeline':
    case 'daily':
    case 'goal_set':
      return stringArg(args, 'day')
    case 'weekly':
      return stringArg(args, 'weekStart')
    case 'card':
    case 'card_update':
    case 'card_delete': {
      const cardId = numberArg(args, 'cardId')
      return cardId === null ? '' : `#${cardId}`
    }
    case 'category_add':
      return stringArg(args, 'name')
    case 'category_update':
    case 'category_remove': {
      const categoryId = stringArg(args, 'categoryId')
      return categoryId === '' ? '' : categoryId.length > 13 ? `${categoryId.slice(0, 13)}…` : categoryId
    }
    default:
      return ''
  }
}

interface ToolOutcome {
  ok: boolean
  code: string
}

/** The result envelope's verdict, or null when there is no result yet. */
function toolOutcome(item: Extract<RenderItem, { kind: 'tool' }>): ToolOutcome | null {
  const result = item.result
  if (result === null) return null
  try {
    const parsed: unknown = JSON.parse(result.content)
    if (typeof parsed === 'object' && parsed !== null && 'ok' in parsed) {
      const envelope = parsed as { ok: unknown; error?: unknown }
      if (envelope.ok === true) return { ok: true, code: '' }
      let code = ''
      if (typeof envelope.error === 'object' && envelope.error !== null && 'code' in envelope.error) {
        const raw = (envelope.error as { code: unknown }).code
        if (typeof raw === 'string') code = raw
      }
      return { ok: false, code }
    }
  } catch {
    // Unparseable content counts as no verdict rather than an error.
  }
  return null
}

function prettyJSON(raw: string): string {
  if (raw === '') return ''
  try {
    return JSON.stringify(JSON.parse(raw) as unknown, null, 2)
  } catch {
    return raw
  }
}

// ---- composer ----

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
    actionError.value = t('chat.loadError')
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
  <div class="page chat">
    <PageHeader :title="t('chat.title')" />
    <p v-if="actionError || store.refreshFailed" class="chat-error" role="alert">
      {{ actionError || t('chat.loadError') }}
      <button v-if="store.activeId" type="button" class="dg-button" @click="perform(() => store.select(store.activeId!))">{{ t('chat.retry') }}</button>
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

          <div ref="scroller" class="main__messages dg-scroll" :aria-busy="store.loading">
            <p v-if="store.messages.length === 0 && !store.loading" class="main__empty">
              {{ t('chat.emptyConversations') }}
            </p>
            <template v-for="item in renderItems" :key="item.kind === 'message' ? item.message.id : groupKey(item)">
              <div
                v-if="item.kind === 'message'"
                class="bubble"
                :class="`bubble--${item.message.role}`"
              >
                <p class="bubble__text">{{ item.message.content }}</p>
                <span
                  v-if="item.message.role === 'assistant' && item.message.status !== 'ok' && item.message.status !== ''"
                  class="bubble__status"
                  :class="`bubble__status--${item.message.status}`"
                >
                  {{ statusLabel(item.message.status) }}
                </span>
              </div>

              <div v-else class="tool-group">
                <button
                  type="button"
                  class="tool-group__head"
                  :aria-expanded="isToolExpanded(item)"
                  :title="isToolExpanded(item) ? t('chat.tools.collapse') : t('chat.tools.expand')"
                  @click="toggleTool(item)"
                >
                  <span class="tool-group__chevron" aria-hidden="true">
                    {{ isToolExpanded(item) ? '▾' : '▸' }}
                  </span>
                  <span class="tool-group__label">{{ toolLabel(item) }}</span>
                  <span v-if="toolDetail(item) !== ''" class="tool-group__detail">
                    {{ toolDetail(item) }}
                  </span>
                  <span
                    v-if="toolOutcome(item) !== null"
                    class="tool-group__status"
                    :class="toolOutcome(item)?.ok ? 'tool-group__status--ok' : 'tool-group__status--error'"
                  >
                    {{ toolOutcome(item)?.ok
                      ? t('chat.tools.resultOk')
                      : toolOutcome(item)?.code !== ''
                        ? `${t('chat.tools.resultError')}（${toolOutcome(item)?.code}）`
                        : t('chat.tools.resultError') }}
                  </span>
                </button>
                <div v-if="isToolExpanded(item)" class="tool-group__body">
                  <section v-if="item.call !== null && prettyJSON(item.call.toolArguments) !== ''">
                    <p class="tool-group__section-label">{{ t('chat.tools.arguments') }}</p>
                    <pre class="tool-group__pre">{{ prettyJSON(item.call.toolArguments) }}</pre>
                  </section>
                  <section v-if="item.result !== null && prettyJSON(item.result.content) !== ''">
                    <p class="tool-group__section-label">{{ t('chat.tools.result') }}</p>
                    <pre class="tool-group__pre">{{ prettyJSON(item.result.content) }}</pre>
                  </section>
                </div>
              </div>
            </template>
          </div>

          <p v-if="store.pending" class="working" role="status">
            <span class="working__spinner" aria-hidden="true" />
            {{ t('chat.working') }}
            <span v-if="effectiveModel !== ''" class="working__model">{{ effectiveModel }}</span>
          </p>
          <p v-if="tooLong" class="chat-error" role="alert">{{ t('chat.tooLong') }}</p>
          <div class="composer-bar">
            <label class="composer-bar__field">
              <span class="dg-field-label">{{ t('chat.provider.label') }}</span>
              <select
                class="dg-input"
                :value="store.activeConversation.providerId"
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
                :model-value="store.activeConversation.model"
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

        <p v-else class="main__empty">
          {{ t('chat.noSelection') }}
        </p>
      </section>

      <!-- Conversation navigation -->
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
    </div>
  </div>
</template>

<style scoped>
.chat-error { color: var(--dg-danger); padding: 0 var(--dg-page-padding); font-size: 12px; }

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

/* ---- conversation navigation (left) ---- */

.side {
  display: flex;
  flex-direction: column;
  order: -1;
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

.main__model {
  width: 200px;
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

/* ---- tool groups ---- */

.tool-group {
  align-self: stretch;
  border: 1px solid var(--dg-card-border);
  border-radius: 8px;
  background: var(--dg-track-fill);
  font-size: 12px;
  overflow: hidden;
}

.tool-group__head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 32px;
  padding: 4px 10px;
  border: none;
  background: none;
  color: var(--dg-text-secondary);
  cursor: pointer;
  text-align: left;
}

.tool-group__head:hover {
  background: var(--dg-hover-fill);
}

.tool-group__head:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--dg-focus-ring);
}

.tool-group__chevron {
  flex: none;
  color: var(--dg-text-muted);
  font-size: 10px;
}

.tool-group__label {
  flex: none;
  color: var(--dg-text-secondary);
  font-weight: 600;
}

.tool-group__detail {
  min-width: 0;
  color: var(--dg-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tool-group__status {
  flex: none;
  margin-left: auto;
  font-size: 10px;
  font-weight: 600;
}

.tool-group__status--ok {
  color: var(--dg-accent-text);
}

.tool-group__status--error {
  color: var(--dg-danger);
}

.tool-group__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px 10px 10px 22px;
  border-top: 1px solid var(--dg-card-border);
}

.tool-group__section-label {
  margin: 0 0 3px;
  color: var(--dg-text-muted);
  font-size: 10px;
  font-weight: 600;
}

.tool-group__pre {
  margin: 0;
  padding: 8px;
  border-radius: 6px;
  background: var(--dg-panel-fill);
  color: var(--dg-text-secondary);
  font-size: 11px;
  line-height: 1.5;
  white-space: pre;
  overflow-x: auto;
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

/* ---- narrow ---- */

@media (max-width: 900px) {
  .layout {
    flex-direction: column;
  }

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
