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
export const useChatStore = defineStore('chat', () => {
  const conversations = ref<ChatConversationDTO[]>([])
  const activeId = ref<string | null>(null)
  const messages = ref<ChatMessageDTO[]>([])
  /** True between SendChatMessage and the completion event for the active
   * conversation; drives the composer's cancel state. */
  const pending = ref(false)
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

  async function loadMessages(conversationId: string): Promise<void> {
    messages.value = await getChatMessages(conversationId, 0, PAGE_SIZE)
  }

  async function refreshConversations(): Promise<void> {
    conversations.value = await listChatConversations()
  }

  async function select(conversationId: string): Promise<void> {
    activeId.value = conversationId
    pending.value = false
    await loadMessages(conversationId)
    await ensureProvider()
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

    try {
      await refreshConversations()
      providers.value = (await listProviders()).map((provider) => ({
        id: provider.id,
        displayName: provider.displayName,
      }))
    } catch {
      unavailable.value = true
      return
    }

    unsubscribe?.()
    unsubscribe = onChatUpdated(async (conversationId) => {
      await refreshConversations()
      if (conversationId === activeId.value) {
        await loadMessages(conversationId)
        pending.value = false
      }
    })

    // Entering the chat view lands on a fresh conversation screen, not an
    // empty state; past threads are one click away in the sidebar.
    await openDraft()
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

  async function send(content: string): Promise<void> {
    const conversationId = activeId.value
    const text = content.trim()
    if (conversationId === null || text === '') return
    // Provider choice is explicit: a thread without one cannot send.
    if (activeConversation.value?.providerId === '') return

    await sendChatMessage(conversationId, text)
    pending.value = true
    // The user message is already committed server-side; re-pull now so it
    // appears immediately rather than after the turn completes. The list
    // refreshes too: the conversation may have just taken its title.
    await Promise.all([loadMessages(conversationId), refreshConversations()])
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
})
