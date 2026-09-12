import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type { ChatConversationDTO, ChatMessageDTO } from '@/api/dto'
import {
  cancelChatTurn,
  createChatConversation,
  deleteChatConversation,
  getChatMessages,
  listChatConversations,
  onChatUpdated,
  sendChatMessage,
  setChatConversationProvider,
} from '@/api/chat'
import { listProviders } from '@/api/providers'
import { updateSettings } from '@/api/settings'

/** Newest messages kept per conversation on first load. */
const PAGE_SIZE = 50

/**
 * Chat conversations, the active thread's transcript, and the global memory.
 *
 * Events are invalidation-only: onChatUpdated triggers a re-pull, never an
 * optimistic insert (docs/05 §5.5.3). The global memory edit goes through
 * UpdateSettings({chat: {memory}}) — it is a setting, and the settings event
 * is the refresh signal for anyone who cares.
 */
const chatAPI = {
  cancelChatTurn,
  createChatConversation,
  deleteChatConversation,
  getChatMessages, listChatConversations, onChatUpdated, sendChatMessage,
  setChatConversationProvider, listProviders, updateSettings,
}

export function createChatState(overrides: Partial<typeof chatAPI> = {}) {
  const {
    cancelChatTurn, createChatConversation, deleteChatConversation,
    getChatMessages, listChatConversations, onChatUpdated, sendChatMessage,
    setChatConversationProvider, listProviders, updateSettings,
  } = { ...chatAPI, ...overrides }
  const conversations = ref<ChatConversationDTO[]>([])
  const activeId = ref<string | null>(null)
  const messages = ref<ChatMessageDTO[]>([])
  /** True while submission is entering the backend or the transcript ends in
   * a non-terminal row. Tool invalidations therefore keep Stop available. */
  const submitting = ref(new Set<string>())
  const loading = ref(false)
  const refreshFailed = ref(false)
  const pending = computed(() => {
    if (activeId.value !== null && submitting.value.has(activeId.value)) return true
    const last = messages.value.at(-1)
    return last !== undefined && last.role !== 'assistant'
  })
  const providers = ref<{ id: string; displayName: string }[]>([])

  const hydrated = ref(false)
  const unavailable = ref(false)

  const activeConversation = computed(() =>
    conversations.value.find((conversation) => conversation.id === activeId.value) ?? null,
  )

  /**
   * Conversations visible in the sidebar: only threads that already carry a
   * message. A thread takes its title from its first user message, so
   * "has a title" is exactly "has been talked to"; fresh drafts stay off the
   * list until then.
   */
  const listedConversations = computed(() =>
    conversations.value.filter((conversation) => conversation.title !== ''),
  )

  let unsubscribe: (() => void) | null = null

  let messageRequest = 0
  let conversationRequest = 0
  async function loadMessages(conversationId: string): Promise<void> {
    if (conversationId !== activeId.value) return
    const request = ++messageRequest
    try {
      const rows = await getChatMessages(conversationId, 0, PAGE_SIZE)
      if (request === messageRequest && conversationId === activeId.value) {
        messages.value = rows
        refreshFailed.value = false
      }
    } finally {
      if (request === messageRequest) loading.value = false
    }
  }

  async function refreshConversations(): Promise<void> {
    const request = ++conversationRequest
    const rows = await listChatConversations()
    if (request === conversationRequest) conversations.value = rows
  }

  async function select(conversationId: string): Promise<void> {
    activeId.value = conversationId
    messages.value = []
    loading.value = true
    await loadMessages(conversationId)
    if (activeId.value === conversationId) await ensureProvider()
  }

  /**
   * A thread must never sit on an empty provider slot: if the pinned provider
   * was deleted (or never set), pin the first configured one.
   */
  async function ensureProvider(): Promise<void> {
    const conversation = activeConversation.value
    if (conversation === null || conversation.providerId !== '') return
    if (providers.value.length === 0) return
    await pinProvider(providers.value[0].id)
  }

  /**
   * Open a fresh conversation screen: reuse the newest existing draft (an
   * untitled thread), or create one. At most one draft exists at a time.
   */
  async function openDraft(): Promise<void> {
    const draft = conversations.value.find((conversation) => conversation.title === '')
    if (draft !== undefined) {
      await select(draft.id)
      return
    }
    const conversation = await createChatConversation()
    await refreshConversations()
    await select(conversation.id)
  }

  async function hydrate(): Promise<void> {
    if (hydrated.value) return
    hydrated.value = true
    unavailable.value = false

    try {
      await refreshConversations()
      providers.value = (await listProviders()).map((provider) => ({
        id: provider.id,
        displayName: provider.displayName,
      }))
    } catch {
      unavailable.value = true
      hydrated.value = false
      return
    }

    unsubscribe?.()
    unsubscribe = onChatUpdated(async (conversationId) => {
      try {
        await Promise.all([refreshConversations(), loadMessages(conversationId)])
      } catch {
        refreshFailed.value = true
      }
    })

    // Entering the chat view lands on a fresh conversation screen, not an
    // empty state; past threads are one click away in the sidebar.
    try {
      await openDraft()
    } catch {
      unavailable.value = true
      hydrated.value = false
    }
  }

  /** New conversation: a no-op when the current screen is already a fresh
   * draft, so repeated clicks never accumulate empty threads. */
  async function newConversation(): Promise<void> {
    if (activeConversation.value?.title === '') return
    await openDraft()
  }

  async function removeConversation(id: string): Promise<void> {
    await deleteChatConversation(id)
    await refreshConversations()
    if (id === activeId.value) await openDraft()
  }

  async function send(content: string): Promise<boolean> {
    const conversationId = activeId.value
    const text = content.trim()
    if (
      conversationId === null || text === '' || pending.value ||
      loading.value || refreshFailed.value
    ) return false
    if (!activeConversation.value?.providerId) return false
    submitting.value.add(conversationId)
    try {
      await sendChatMessage(conversationId, text)
      // A successful binding means the user message is committed. A read
      // failure must not invite resending that already accepted message.
      try {
        await Promise.all([loadMessages(conversationId), refreshConversations()])
      } catch {
        refreshFailed.value = true
      }
      return true
    } finally {
      submitting.value.delete(conversationId)
    }
  }

  async function cancel(): Promise<void> {
    if (activeId.value === null) return
    await cancelChatTurn(activeId.value)
  }

  async function pinProvider(providerId: string): Promise<void> {
    const conversationId = activeId.value
    if (conversationId === null) return
    await setChatConversationProvider(conversationId, providerId)
    await refreshConversations()
  }

  /** Persist the global chat memory (like a CLAUDE.md). */
  async function saveMemory(memory: string): Promise<void> {
    await updateSettings({ chatMemory: memory })
  }

  return {
    conversations,
    loading,
    refreshFailed,
    refreshConversations,
    listedConversations,
    activeId,
    activeConversation,
    messages,
    pending,
    providers,
    unavailable,
    hydrate,
    select,
    newConversation,
    removeConversation,
    send,
    cancel,
    pinProvider,
    saveMemory,
  }
}

export const useChatStore = defineStore('chat', () => createChatState())
