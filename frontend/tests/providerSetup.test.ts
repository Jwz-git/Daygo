import assert from 'node:assert/strict'
import test from 'node:test'
import { createRenderer, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import ProviderSetup from '../src/views/Settings/ProviderSetup.vue'
import zhCN from '../src/locales/zh-CN'
import type { ChatGPTConnection, ProviderDTO } from '../src/api/dto'

interface Node {
  kind: string
  text: string
  children: Node[]
  parent: Node | null
  props: Record<string, unknown>
  tagName: string
  addEventListener: () => void
  removeEventListener: () => void
  scrollIntoView: () => void
  focus: () => void
}
function node(kind: string, text = ''): Node { return { kind, text, children: [], parent: null, props: {}, tagName: kind.toUpperCase(), addEventListener() {}, removeEventListener() {}, scrollIntoView() {}, focus() {} } }
function all(root: Node): Node[] { return [root, ...root.children.flatMap(all)] }
function text(root: Node): string { return root.kind === 'comment' ? '' : root.text + root.children.map(text).join('') }
async function settle(): Promise<void> { for (let i = 0; i < 8; i++) await Promise.resolve(); await nextTick(); await nextTick() }

function mount(provider: ProviderDTO | null = null, begin?: () => Promise<string>) {
  const previousWindow = globalThis.window
  const previousDocument = globalThis.document
  globalThis.window = new EventTarget() as unknown as Window & typeof globalThis
  let starts = 0
  const canceled: string[] = []
  const reads: string[] = []
  Object.assign(globalThis.window, { go: { app: { Backend: {
    async BeginChatGPTLogin() { starts++; return begin ? begin() : 'fixture' },
    async CancelChatGPTLogin(id: string) { canceled.push(id) },
    async GetChatGPTConnection(id: string): Promise<ChatGPTConnection> {
      reads.push(id)
      return { providerId: id, state: provider ? 'connected' : 'waiting', errorCode: '', email: '', models: [], paused: false, needsConfirmation: false }
    },
  } } } })
  globalThis.document = new EventTarget() as unknown as Document
  const body = node('body')
  const renderer = createRenderer<Node, Node>({
    querySelector: () => body,
    createElement: node, createText: (value) => node('text', value), createComment: (value) => node('comment', value),
    insert(child, parent, anchor = null) {
      if (child.parent) child.parent.children.splice(child.parent.children.indexOf(child), 1)
      child.parent = parent
      const index = anchor ? parent.children.indexOf(anchor) : -1
      if (index < 0) parent.children.push(child); else parent.children.splice(index, 0, child)
    },
    remove(child) { if (child.parent) child.parent.children.splice(child.parent.children.indexOf(child), 1) },
    setText: (element, value) => { element.text = value },
    setElementText: (element, value) => { element.text = value; element.children = [] },
    parentNode: (element) => element.parent,
    nextSibling: (element) => element.parent?.children[element.parent.children.indexOf(element) + 1] ?? null,
    patchProp: (element, key, _old, value) => { element.props[key] = value },
  })
  const root = node('root')
  const app = renderer.createApp(ProviderSetup, { provider })
  app.use(createPinia()).use(createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN } }))
  app.mount(root)
  return {
    root,
    starts: () => starts,
    canceled,
    reads,
    buttons: () => all(root).filter((element) => element.kind === 'button').map(text),
    async click(label: string) {
      const button = all(root).find((element) => element.kind === 'button' && text(element).trim() === label)
      assert.ok(button, `missing ${label}`)
      assert.equal(typeof button.props.onClick, 'function')
      await (button.props.onClick as () => unknown)(); await settle()
    },
    dispose() { app.unmount(); globalThis.window = previousWindow; globalThis.document = previousDocument },
  }
}

test('new provider starts with exactly the two requested choices and no expanded form', () => {
  const view = mount()
  try {
    assert.deepEqual(view.buttons(), ['ChatGPT 订阅', '配置API Key'])
    assert.equal(all(view.root).some((element) => element.kind === 'form'), false)
    assert.equal(all(view.root).some((element) => element.kind === 'a'), false)
  } finally { view.dispose() }
})

test('subscription choice starts authorization once without continue or usage links; back cancels it', async () => {
  const view = mount()
  try {
    await view.click('ChatGPT 订阅')
    assert.equal(view.starts(), 1)
    assert.ok(text(view.root).includes('请在浏览器中完成授权'))
    assert.equal(text(view.root).includes('继续'), false)
    assert.equal(all(view.root).some((element) => element.kind === 'a'), false)
    assert.equal(text(view.root).includes('配置API Key'), false)
    await view.click('返回')
    assert.deepEqual(view.canceled, ['fixture'])
    assert.deepEqual(view.buttons(), ['ChatGPT 订阅', '配置API Key'])
    await view.click('配置API Key')
    assert.ok(text(view.root).includes('接口类型'))
    assert.equal(view.starts(), 1)
    await view.click('返回')
    assert.deepEqual(view.buttons(), ['ChatGPT 订阅', '配置API Key'])
  } finally { view.dispose() }
})

test('failed automatic authorization offers an explicit retry', async () => {
  let attempts = 0
  const view = mount(null, async () => { if (++attempts === 1) throw new Error('anonymous failure'); return 'retry' })
  try {
    await view.click('ChatGPT 订阅')
    assert.equal(view.starts(), 1)
    assert.ok(view.buttons().includes('重新授权'))
    await view.click('重新授权')
    assert.equal(view.starts(), 2)
    assert.ok(text(view.root).includes('请在浏览器中完成授权'))
  } finally { view.dispose() }
})

test('back during authorization startup cancels a late attempt without polling it', async () => {
  let resolve!: (id: string) => void
  const view = mount(null, () => new Promise<string>((done) => { resolve = done }))
  try {
    await view.click('ChatGPT 订阅')
    assert.equal(view.starts(), 1)
    await view.click('返回')
    resolve('late')
    await settle()
    assert.deepEqual(view.canceled, ['late'])
    assert.deepEqual(view.reads, [])
    assert.deepEqual(view.buttons(), ['ChatGPT 订阅', '配置API Key'])
  } finally { view.dispose() }
})

test('editing a saved subscription reads its status without automatically reauthorizing', async () => {
  const view = mount({ id: 'saved', displayName: 'Anonymous', protocol: 'chatgpt', endpoint: 'https://api.openai.com/v1', models: ['fixture'], maxImages: 0, userAgent: '', hasSecret: true })
  try {
    await settle()
    assert.equal(view.starts(), 0)
    assert.deepEqual(view.reads, ['saved'])
    assert.ok(view.buttons().includes('重新授权'))
    assert.equal(all(view.root).some((element) => element.kind === 'a'), false)
  } finally { view.dispose() }
})

test('editing an API key provider opens its existing form directly', () => {
  const view = mount({ id: 'fixture', displayName: 'Anonymous', protocol: 'openai', endpoint: 'https://example.invalid/v1', models: ['fixture'], maxImages: 0, userAgent: '', hasSecret: false })
  try {
    assert.ok(all(view.root).some((element) => element.kind === 'form'))
    assert.equal(view.buttons().includes('ChatGPT 订阅'), false)
    assert.equal(view.buttons().includes('配置API Key'), false)
  } finally { view.dispose() }
})
