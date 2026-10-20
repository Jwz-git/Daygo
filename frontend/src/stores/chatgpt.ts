import { computed, ref } from 'vue'
import * as api from '@/api/chatgpt'
import type { ChatGPTConnection } from '@/api/dto'

export type ChatGPTAPI = typeof api

/** One owned setup session. OAuth values never reach this layer. */
export function createChatGPTSetup(deps: ChatGPTAPI = api, savedId = '') {
  const connection = ref<ChatGPTConnection | null>(null)
  const working = ref(false)
  const failed = ref(false)
  const remoteUnconfirmed = ref(false)
  const waiting = computed(() => connection.value?.state === 'waiting')
  let pendingId = ''
  let disposed = false
  let revision = 0
  let loadSequence = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  const currentId = () => pendingId || savedId
  const stopTimer = () => { clearTimeout(timer); timer = undefined }

  async function load(): Promise<void> {
    const id = currentId(), version = revision, sequence = ++loadSequence
    if (!id || disposed) return
    stopTimer()
    try {
      const next = await deps.getChatGPTConnection(id)
      if (disposed || version !== revision || sequence !== loadSequence) return
      connection.value = next
      if (next.state === 'waiting') timer = setTimeout(() => { void load() }, 750)
    } catch { if (!disposed && version === revision && sequence === loadSequence) failed.value = true }
  }

  async function cancel(): Promise<void> {
    revision++; stopTimer()
    const id = pendingId; pendingId = ''
    if (id) await deps.cancelChatGPTLogin(id)
  }

  async function start(): Promise<void> {
    if (working.value || disposed) return
    working.value = true; failed.value = false
    try {
      await cancel()
      const version = revision
      const id = await deps.beginChatGPTLogin(savedId)
      if (disposed || version !== revision) { await deps.cancelChatGPTLogin(id); return }
      pendingId = id
      await load()
    } catch { if (!disposed) failed.value = true }
    finally { working.value = false }
  }

  async function confirm(): Promise<void> {
    if (!currentId() || working.value) return
    working.value = true; failed.value = false
    try { await deps.confirmChatGPTPlan(currentId()); await load() }
    catch { failed.value = true }
    finally { working.value = false }
  }

  async function save(name: string, models: string[]): Promise<boolean> {
    const status = connection.value
    if (!status || !['ready', 'connected'].includes(status.state) || status.needsConfirmation || !name.trim() || models.length === 0 || working.value) return false
    working.value = true; failed.value = false
    try {
      await deps.saveChatGPTProvider(currentId(), name.trim(), models)
      pendingId = ''; stopTimer()
      return true
    } catch { failed.value = true; return false }
    finally { working.value = false }
  }

  async function disconnect(): Promise<void> {
    if (!currentId() || working.value) return
    const id = currentId()
    working.value = true; failed.value = false
    try { remoteUnconfirmed.value = !await deps.disconnectChatGPT(id); await load() }
    catch { failed.value = true }
    finally { working.value = false }
  }

  async function resume(): Promise<void> {
    if (!currentId() || working.value) return
    working.value = true; failed.value = false
    try { await deps.resumeChatGPTSubscription(currentId()); await load() }
    catch { failed.value = true }
    finally { working.value = false }
  }

  function dispose(): void {
    disposed = true
    void cancel().catch(() => { /* no UI survives unmount */ })
  }
  return { connection, working, failed, waiting, remoteUnconfirmed, load, start, cancel, confirm, save, disconnect, resume, dispose }
}
