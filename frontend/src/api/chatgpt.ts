import { hasDesktopBridge } from '@/api/desktopBridge'
import {
  BeginChatGPTLogin, CancelChatGPTLogin, GetChatGPTConnection,
  SaveChatGPTProvider, DisconnectChatGPT, ResumeChatGPTSubscription, ConfirmChatGPTPlan,
} from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'
import type { ChatGPTConnection } from './dto'
import { WAILS_UNAVAILABLE } from './providers'

function requireBridge(): void { if (!hasDesktopBridge()) throw new Error(WAILS_UNAVAILABLE) }
// No browser fallback or credential-bearing DTO: OAuth belongs to Go.
export async function beginChatGPTLogin(id: string): Promise<string> { requireBridge(); return BeginChatGPTLogin(id) }
export async function cancelChatGPTLogin(id: string): Promise<void> { requireBridge(); return CancelChatGPTLogin(id) }
export async function getChatGPTConnection(id: string): Promise<ChatGPTConnection> {
  requireBridge()
  const connection = await GetChatGPTConnection(id)
  return { ...connection, models: connection.models ?? [] }
}
export async function saveChatGPTProvider(id: string, name: string, models: string[]): Promise<void> { requireBridge(); return SaveChatGPTProvider(id, name, models) }
export async function disconnectChatGPT(id: string): Promise<boolean> { requireBridge(); return DisconnectChatGPT(id) }
export async function resumeChatGPTSubscription(id: string): Promise<void> { requireBridge(); return ResumeChatGPTSubscription(id) }
export async function confirmChatGPTPlan(id: string): Promise<void> { requireBridge(); return ConfirmChatGPTPlan(id) }
