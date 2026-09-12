import {
  CancelChatTurn,
  CreateChatConversation,
  DeleteChatConversation,
  GetChatMessages,
  ListChatConversations,
  SendChatMessage,
  SetChatConversationProvider,
} from '../../wailsjs/go/app/Backend'

import type { ChatConversationDTO, ChatMessageDTO } from '@/api/dto'
import { listProviders } from '@/api/providers'

/** Thrown when the page runs in a plain browser, outside the Wails WebView. */
export const WAILS_UNAVAILABLE = 'wails_unavailable'

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

/*
 * Dev-browser stand-in: an in-memory chat state so the conversation UI is
 * exercisable outside the WebView. Replies are canned. Dev only.
 */
interface DevConversation {
  dto: ChatConversationDTO
  messages: ChatMessageDTO[]
  nextMessageId: number
}

let devConversations: DevConversation[] | null = null
let devNextConversationId = 1

function devState(): DevConversation[] {
  devConversations ??= []
  return devConversations
}

export async function listChatConversations(): Promise<ChatConversationDTO[]> {
  if (hasBridge()) {
    return (await ListChatConversations()) as unknown as ChatConversationDTO[]
  }
  if (import.meta.env.DEV) {
    return devState().map((conversation) => ({ ...conversation.dto }))
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function createChatConversation(): Promise<ChatConversationDTO> {
  if (hasBridge()) {
    return (await CreateChatConversation()) as unknown as ChatConversationDTO
  }
  if (import.meta.env.DEV) {
    // Mirror the backend: a new thread defaults to the first configured
    // provider (the routing chain's primary).
    const providers = await listProviders()
    const dto: ChatConversationDTO = {
      id: `dev-conv-${devNextConversationId++}`,
      title: '',
      providerId: providers[0]?.id ?? '',
      updatedAt: Math.floor(Date.now() / 1000),
    }
    devState().unshift({ dto, messages: [], nextMessageId: 1 })
    return { ...dto }
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function deleteChatConversation(id: string): Promise<void> {
  if (hasBridge()) return DeleteChatConversation(id)
  if (import.meta.env.DEV) {
    devConversations = devState().filter((conversation) => conversation.dto.id !== id)
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function setChatConversationProvider(id: string, providerId: string): Promise<void> {
  if (hasBridge()) return SetChatConversationProvider(id, providerId)
  if (import.meta.env.DEV) {
    for (const conversation of devState()) {
      if (conversation.dto.id === id) conversation.dto.providerId = providerId
    }
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function getChatMessages(
  conversationId: string,
  beforeId: number,
  limit: number,
): Promise<ChatMessageDTO[]> {
  if (hasBridge()) {
    return (await GetChatMessages(conversationId, beforeId, limit)) as unknown as ChatMessageDTO[]
  }
  if (import.meta.env.DEV) {
    const conversation = devState().find((entry) => entry.dto.id === conversationId)
    if (conversation === undefined) return []
    let messages = [...conversation.messages]
    if (beforeId > 0) messages = messages.filter((message) => message.id < beforeId)
    if (limit > 0) messages = messages.slice(-limit)
    return messages
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function sendChatMessage(conversationId: string, content: string): Promise<void> {
  if (hasBridge()) return SendChatMessage(conversationId, content)
  if (import.meta.env.DEV) {
    const conversation = devState().find((entry) => entry.dto.id === conversationId)
    if (conversation === undefined) throw new Error(WAILS_UNAVAILABLE)
    conversation.messages.push({
      id: conversation.nextMessageId++,
      role: 'user',
      content,
      status: '',
      createdAt: Math.floor(Date.now() / 1000),
    })
    conversation.messages.push({
      id: conversation.nextMessageId++,
      role: 'assistant',
      content: '（开发环境固定回复）当前没有接入真实供应商。',
      status: 'ok',
      createdAt: Math.floor(Date.now() / 1000),
    })
    if (conversation.dto.title === '') conversation.dto.title = content.slice(0, 30)
    conversation.dto.updatedAt = Math.floor(Date.now() / 1000)
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function cancelChatTurn(conversationId: string): Promise<void> {
  if (hasBridge()) return CancelChatTurn(conversationId)
  if (import.meta.env.DEV) return
  throw new Error(WAILS_UNAVAILABLE)
}

interface WailsRuntime {
  EventsOnMultiple: (
    eventName: string,
    callback: (...data: unknown[]) => void,
    maxCallbacks: number,
  ) => () => void
}

function isWailsRuntime(value: unknown): value is WailsRuntime {
  return typeof value === 'object' && value !== null &&
    'EventsOnMultiple' in value && typeof value.EventsOnMultiple === 'function'
}

function chatUpdatedPayload(value: unknown): string | null {
  if (typeof value !== 'object' || value === null || !('conversationId' in value)) return null
  const conversationId = (value as { conversationId: unknown }).conversationId
  return typeof conversationId === 'string' ? conversationId : null
}

/** Subscribe to turn-completion invalidations. The payload is a conversation
 * id only; callers re-pull through getChatMessages. */
export function onChatUpdated(callback: (conversationId: string) => void): () => void {
  if (!('runtime' in window) || !isWailsRuntime(window.runtime)) return () => undefined
  return window.runtime.EventsOnMultiple('chat:updated', (raw: unknown) => {
    const conversationId = chatUpdatedPayload(raw)
    if (conversationId !== null) callback(conversationId)
  }, -1) ?? (() => undefined)
}
