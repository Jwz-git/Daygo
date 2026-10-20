import assert from 'node:assert/strict'
import test from 'node:test'
import { createRenderer, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import ProviderSetup from '../src/views/Settings/ProviderSetup.vue'
import zhCN from '../src/locales/zh-CN'
import type { ProviderDTO } from '../src/api/dto'

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
async function settle(): Promise<void> { await Promise.resolve(); await nextTick(); await nextTick() }

function mount(provider: ProviderDTO | null = null) {
  const previousWindow = globalThis.window
  const previousDocument = globalThis.document
  globalThis.window = new EventTarget() as unknown as Window & typeof globalThis
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

test('choosing a method opens only that form; back restores the two choices', async () => {
  const view = mount()
  try {
    await view.click('ChatGPT 订阅')
    assert.ok(text(view.root).includes('使用 ChatGPT 继续'))
    assert.equal(text(view.root).includes('配置API Key'), false)
    await view.click('返回')
    assert.deepEqual(view.buttons(), ['ChatGPT 订阅', '配置API Key'])
    await view.click('配置API Key')
    assert.ok(text(view.root).includes('接口类型'))
    assert.equal(text(view.root).includes('使用 ChatGPT 继续'), false)
    await view.click('返回')
    assert.deepEqual(view.buttons(), ['ChatGPT 订阅', '配置API Key'])
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
