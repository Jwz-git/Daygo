import assert from 'node:assert/strict'
import test from 'node:test'
import { createChatGPTSetup, type ChatGPTAPI } from '../src/stores/chatgpt'
import type { ChatGPTConnection } from '../src/api/dto'

const connected: ChatGPTConnection = { providerId: 'fixture', state: 'ready', errorCode: '', email: 'anonymous@example.invalid', models: ['fixture-model'], paused: false, needsConfirmation: true }
function fixture(overrides: Partial<ChatGPTAPI> = {}) {
  let canceled = 0, saved = 0, confirmed = false
  const api: ChatGPTAPI = {
    beginChatGPTLogin: async () => 'fixture', cancelChatGPTLogin: async () => { canceled++ },
    getChatGPTConnection: async () => ({ ...connected, needsConfirmation: !confirmed }),
    saveChatGPTProvider: async () => { saved++ }, disconnectChatGPT: async () => false,
    resumeChatGPTSubscription: async () => {}, confirmChatGPTPlan: async () => { confirmed = true },
    ...overrides,
  }
  return { setup: createChatGPTSetup(api), canceled: () => canceled, saved: () => saved }
}

test('subscription requires explicit first-use confirmation before saving', async () => {
  const f = fixture()
  await f.setup.start()
  assert.equal(f.setup.connection.value?.state, 'ready')
  assert.equal(await f.setup.save('fixture', ['fixture-model']), false)
  assert.equal(f.saved(), 0)
  await f.setup.confirm()
  assert.equal(await f.setup.save('fixture', ['fixture-model']), true)
  assert.equal(f.saved(), 1)
  f.setup.dispose()
})

test('unmount during login cancels the late attempt and never starts polling', async () => {
  let resolve!: (id: string) => void
  let reads = 0
  const f = fixture({ beginChatGPTLogin: () => new Promise<string>((done) => { resolve = done }), getChatGPTConnection: async () => { reads++; return connected } })
  const starting = f.setup.start()
  await Promise.resolve() // let begin enter its pending request
  f.setup.dispose()
  resolve('fixture')
  await starting
  assert.equal(f.canceled(), 1)
  assert.equal(reads, 0)
})

test('failed save preserves model selection and remote revoke warning stays visible', async () => {
  const f = fixture({ saveChatGPTProvider: async () => { throw new Error('anonymous failure') } })
  await f.setup.start(); await f.setup.confirm()
  assert.equal(await f.setup.save('fixture', ['fixture-model']), false)
  assert.equal(f.setup.connection.value?.state, 'ready')
  await f.setup.disconnect()
  assert.equal(f.setup.remoteUnconfirmed.value, true)
  f.setup.dispose()
})
