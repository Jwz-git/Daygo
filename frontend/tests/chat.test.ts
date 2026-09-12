import assert from 'node:assert/strict'
import test from 'node:test'
import { createChatState } from '../src/stores/chat'
import type { ChatMessageDTO } from '../src/api/dto'

const row = (role: string, id = 1): ChatMessageDTO => ({ id, role, content: role, status: role === 'assistant' ? 'ok' : '', toolName: '', toolArguments: '', createdAt: 0 })
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}
function fixture() {
  let rows: ChatMessageDTO[] = []
  let listener: (id: string) => Promise<void> = async () => {}
  let sends = 0
  const state = createChatState({
    listChatConversations: async () => ['a', 'b'].map((id) => ({ id, title: id, providerId: 'p', updatedAt: 0 })),
    getChatMessages: async () => rows,
    listProviders: async () => [],
    onChatUpdated: (callback) => { listener = callback; return () => {} },
    sendChatMessage: async () => { sends++; rows = [row('user')]; await listener('a') },
  })
  return { state, setRows: (value: ChatMessageDTO[]) => { rows = value }, update: (id: string) => listener(id), sends: () => sends }
}

test('tool invalidations keep Stop available; terminal assistant unlocks composer', async () => {
  const f = fixture()
  await f.state.refreshConversations()
  await f.state.select('a')
  await f.state.send('hello')
  assert.equal(f.state.pending.value, true)
  f.setRows([row('user'), row('tool_call', 2), row('tool_result', 3)])
  await f.state.select('b')
  await f.state.select('a')
  assert.equal(f.state.pending.value, true)
  f.setRows([row('assistant', 4)])
  await f.state.select('a')
  assert.equal(f.state.pending.value, false)
})

test('late conversation response cannot overwrite a newer selection', async () => {
  const slow = deferred<ChatMessageDTO[]>()
  const state = createChatState({ getChatMessages: async (id) => id === 'a' ? slow.promise : [row('assistant', 2)] })
  const first = state.select('a')
  await state.select('b')
  slow.resolve([row('user')])
  await first
  assert.equal(state.activeId.value, 'b')
  assert.equal(state.messages.value[0].id, 2)
})

test('submission is locked before binding resolves and failures release it', async () => {
  const slow = deferred<void>()
  let sends = 0
  const f = fixture()
  await f.state.refreshConversations()
  const state = createChatState({ sendChatMessage: async () => { sends++; await slow.promise; throw new Error('failure') } })
  state.conversations.value = f.state.conversations.value
  state.activeId.value = 'a'
  const first = state.send('hello')
  assert.equal(state.pending.value, true)
  assert.equal(await state.send('again'), false)
  slow.resolve()
  await assert.rejects(first)
  assert.equal(state.pending.value, false)
  assert.equal(sends, 1)
})
