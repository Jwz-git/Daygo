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
import { canUseDevelopmentTestData } from '@/api/developmentFixtures'
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
  if (import.meta.env.DEV && canUseDevelopmentTestData()) {
    return devState().map((conversation) => ({ ...conversation.dto }))
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function createChatConversation(): Promise<ChatConversationDTO> {
  if (hasBridge()) {
    return (await CreateChatConversation()) as unknown as ChatConversationDTO
  }
  if (import.meta.env.DEV && canUseDevelopmentTestData()) {
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
  if (import.meta.env.DEV && canUseDevelopmentTestData()) {
    devConversations = devState().filter((conversation) => conversation.dto.id !== id)
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function setChatConversationProvider(id: string, providerId: string): Promise<void> {
  if (hasBridge()) return SetChatConversationProvider(id, providerId)
  if (import.meta.env.DEV && canUseDevelopmentTestData()) {
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
  if (import.meta.env.DEV && canUseDevelopmentTestData()) {
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
  if (import.meta.env.DEV && canUseDevelopmentTestData()) {
    const conversation = devState().find((entry) => entry.dto.id === conversationId)
    if (conversation === undefined) throw new Error(WAILS_UNAVAILABLE)
    const now = Math.floor(Date.now() / 1000)
    const nextId = (): number => conversation.nextMessageId++
    conversation.messages.push({
      id: nextId(),
      role: 'user',
      content,
      status: '',
      toolName: '',
      toolArguments: '',
      createdAt: now,
    })
    // One scripted tool turn so the collapsed tool rendering is exercisable
    // in the dev browser, mirroring the real agent transcript shape.
    conversation.messages.push({
      id: nextId(),
      role: 'tool_call',
      content: '',
      status: '',
      toolName: 'timeline',
      toolArguments: '{"day":"2026-09-12"}',
      createdAt: now,
    })
    conversation.messages.push({
      id: nextId(),
      role: 'tool_result',
      content: '{"ok":true,"data":{"day":"2026-09-12","cards":[]}}',
      status: '',
      toolName: 'timeline',
      toolArguments: '',
      createdAt: now,
    })
    conversation.messages.push({
      id: nextId(),
      role: 'assistant',
      content: '（开发环境固定回复）当前没有接入真实供应商。',
      status: 'ok',
      toolName: '',
      toolArguments: '',
      createdAt: now,
    })
    if (conversation.dto.title === '') conversation.dto.title = content.slice(0, 30)
    conversation.dto.updatedAt = Math.floor(Date.now() / 1000)
    devNotifyChatUpdated(conversationId)
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function cancelChatTurn(conversationId: string): Promise<void> {
  if (hasBridge()) return CancelChatTurn(conversationId)
  if (import.meta.env.DEV && canUseDevelopmentTestData()) return
  throw new Error(WAILS_UNAVAILABLE)
}

/**
 * Dev-browser stand-in for the chat:updated event. The store resets its
 * pending flag only on that event; without a stand-in the dev composer locks
 * after one message per page load.
 */
let devChatUpdatedListeners: ((conversationId: string) => void)[] = []

function devNotifyChatUpdated(conversationId: string): void {
  // A macrotask: the store sets pending=true only after sendChatMessage
  // resolves, so the notification must land after that continuation.
  setTimeout(() => {
    for (const callback of [...devChatUpdatedListeners]) callback(conversationId)
  }, 0)
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

/** Subscribe to conversation and message invalidations. The payload is a conversation
 * id only; callers re-pull through getChatMessages. */
export function onChatUpdated(callback: (conversationId: string) => void): () => void {
  if (!('runtime' in window) || !isWailsRuntime(window.runtime)) {
    if (import.meta.env.DEV && canUseDevelopmentTestData()) {
      devChatUpdatedListeners.push(callback)
      return () => {
        devChatUpdatedListeners = devChatUpdatedListeners.filter((entry) => entry !== callback)
      }
    }
    return () => undefined
  }
  return window.runtime.EventsOnMultiple('chat:updated', (raw: unknown) => {
    const conversationId = chatUpdatedPayload(raw)
    if (conversationId !== null) callback(conversationId)
  }, -1) ?? (() => undefined)
}
