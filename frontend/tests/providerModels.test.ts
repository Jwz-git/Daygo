import assert from 'node:assert/strict'
import test from 'node:test'
import { createRenderer, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'

import type { ProviderDTO, ProviderInput, ProviderModelsRequest, ProviderModelsResult } from '../src/api/dto'
import ProviderForm from '../src/views/Settings/ProviderForm.vue'
import zhCN from '../src/locales/zh-CN'

// Exercise the real form and combobox, using Vue's host renderer as in the
// player fixture. No real provider, keychain or user database is touched.
interface HostNode {
  kind: string
  children: HostNode[]
  parent: HostNode | null
  props: Record<string, unknown>
  text: string
  tagName: string
  focus: () => void
  addEventListener: () => void
  removeEventListener: () => void
  scrollIntoView: () => void
}

function node(kind: string, text = ''): HostNode {
  return { kind, text, children: [], parent: null, props: {}, tagName: kind.toUpperCase(),
    focus() {}, addEventListener() {}, removeEventListener() {}, scrollIntoView() {} }
}

function collect(root: HostNode): HostNode[] {
  return [root, ...root.children.flatMap(collect)]
}

function content(root: HostNode): string {
  if (root.kind === 'comment' || root.kind === 'svg') return ''
  return root.text + root.children.map(content).join('')
}

function event(element: HostNode, name: string, payload: unknown = {}): unknown {
  const handler = element.props[name]
  assert.equal(typeof handler, 'function', `${element.kind} needs ${name}`)
  return (handler as (payload: unknown) => unknown)(payload)
}

const saved: ProviderDTO = {
  id: 'fixture', displayName: 'Anonymous', protocol: 'openai',
  endpoint: 'https://example.invalid/v1', models: ['custom-model'],
  maxImages: 0, userAgent: '', hasSecret: true,
}
const success: ProviderModelsResult = {
  ok: true, models: ['fixture-small', 'fixture-large'], errorCode: '', message: '',
}

function mountForm(list: (request: ProviderModelsRequest) => Promise<ProviderModelsResult>) {
  const previous = { window: globalThis.window, document: globalThis.document }
  let submitted: ProviderInput | undefined
  const requests: ProviderModelsRequest[] = []
  globalThis.window = Object.assign(new EventTarget(), { go: { app: { Backend: {
    ListProviderModels: async (request: ProviderModelsRequest) => {
      requests.push(request)
      return list(request)
    },
    UpdateProvider: async (_id: string, input: ProviderInput) => { submitted = input },
    ListProviders: async () => [saved],
    GetProviderRouting: async () => ({ chain: [] }),
  } } }, setTimeout }) as unknown as typeof globalThis.window
  globalThis.document = new EventTarget() as unknown as Document
  const body = node('body')
  const renderer = createRenderer<HostNode, HostNode>({
    createElement: node, createText: (text) => node('text', text),
    createComment: (text) => node('comment', text),
    insert(child, parent, anchor = null) {
      if (child.parent) child.parent.children.splice(child.parent.children.indexOf(child), 1)
      child.parent = parent
      const index = anchor === null ? -1 : parent.children.indexOf(anchor)
      if (index < 0) parent.children.push(child)
      else parent.children.splice(index, 0, child)
    },
    remove(child) {
      if (child.parent) child.parent.children.splice(child.parent.children.indexOf(child), 1)
      child.parent = null
    },
    setText: (element, text) => { element.text = text },
    setElementText: (element, text) => { element.text = text; element.children = [] },
    parentNode: (element) => element.parent,
    nextSibling: (element) => element.parent?.children[element.parent.children.indexOf(element) + 1] ?? null,
    patchProp: (element, key, _old, value) => { element.props[key] = value },
    querySelector: () => body,
  })
  const root = node('root')
  const app = renderer.createApp(ProviderForm, { provider: saved })
  app.use(createPinia()).use(createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN } }))
  app.mount(root)
  const find = (predicate: (element: HostNode) => boolean): HostNode => {
    const found = collect(root).find(predicate)
    assert.ok(found, 'expected control is mounted')
    return found
  }
  return {
    root, requests,
    find,
    button: (label: string) => find((element) => element.kind === 'button' && content(element).trim() === label),
    combos: () => collect(root).filter((element) => element.kind === 'input' && element.props.role === 'combobox'),
    options: () => collect(root).filter((element) => element.props.role === 'option'),
    submitted: () => submitted,
    dispose() { app.unmount(); Object.assign(globalThis, previous) },
  }
}

async function settle(): Promise<void> {
  await Promise.resolve()
  await nextTick()
  await nextTick()
}

test('fetching populates candidates without adding models; picks and unrelated edits keep them', async () => {
  const form = mountForm(async () => success)
  try {
    await event(form.button('获取模型'), 'onClick')
    await settle()
    assert.deepEqual(form.requests, [{ providerId: saved.id }], 'stored key stays in Go')
    assert.equal(form.combos().length, 1, 'listing must not configure every fetched model')
    assert.equal(form.combos()[0]!.props.value, 'custom-model')
    event(form.combos()[0]!, 'onFocus')
    await settle()
    assert.deepEqual(form.options().map(content), success.models)
    event(form.options()[1]!.children[0]!, 'onClick')
    await settle()
    assert.equal(form.combos()[0]!.props.value, 'fixture-large')
    const name = form.find((element) => element.props.placeholder === '例如：公司网关')
    event(name, 'onUpdate:modelValue', 'Renamed')
    await event(form.button('添加模型'), 'onClick')
    await settle()
    event(form.combos()[1]!, 'onFocus')
    await settle()
    assert.deepEqual(form.options().map(content), success.models)
  } finally { form.dispose() }
})

test('typing, clearing and immediately saving use the current combobox value', async () => {
  for (const typed of ['manual-model', '']) {
    const form = mountForm(async () => success)
    try {
      const combo = form.combos()[0]!
      event(combo, 'onFocus')
      await settle()
      assert.equal(combo.props.value, 'custom-model', 'focusing must preserve the editable value')
      event(combo, 'onInput', { target: { value: typed } })
      await event(form.find((element) => element.kind === 'form'), 'onSubmit', { preventDefault() {} })
      await settle()
      if (typed !== '') assert.deepEqual(form.submitted()?.models, [typed])
      else {
        assert.equal(form.submitted(), undefined, 'a cleared last model must fail required validation')
        assert.ok(content(form.root).includes('不能为空'))
      }
    } finally { form.dispose() }
  }
})

test('keyboard filters and selects a candidate without submitting the form', async () => {
  const form = mountForm(async () => success)
  try {
    await event(form.button('获取模型'), 'onClick')
    await settle()
    const combo = form.combos()[0]!
    event(combo, 'onFocus')
    event(combo, 'onInput', { target: { value: 'large' } })
    await settle()
    assert.deepEqual(form.options().map(content), ['fixture-large'])
    let prevented = 0
    const key = (value: string) => ({ key: value, preventDefault() { prevented++ } })
    event(combo, 'onKeydown', key('ArrowDown'))
    await settle()
    event(combo, 'onKeydown', key('Enter'))
    await settle()
    assert.equal(combo.props.value, 'fixture-large')
    assert.equal(combo.props['aria-expanded'], false)
    assert.equal(prevented, 2)
    assert.equal(form.submitted(), undefined)
  } finally { form.dispose() }
})

test('changing source fields clears candidates and discards results from the previous source', async () => {
  for (const field of ['endpoint', 'secret', 'userAgent'] as const) {
    let resolve!: (result: ProviderModelsResult) => void
    const form = mountForm(() => new Promise((done) => { resolve = done }))
    try {
      const pending = event(form.button('获取模型'), 'onClick')
      await settle()
      const control = form.find((element) => element.kind === 'input' && (field === 'endpoint' ? element.props.type === 'url'
          : field === 'secret' ? element.props.type === 'password' : element.props.placeholder === '留空则使用默认值'))
      event(control, 'onUpdate:modelValue', field === 'endpoint'
        ? 'https://other.invalid/v1' : field === 'secret' ? 'fixture-key' : 'Fixture/1')
      await settle()
      resolve(success)
      await pending
      await settle()
      event(form.combos()[0]!, 'onFocus')
      await settle()
      assert.deepEqual(form.options(), [], `${field}: late response must not repopulate the list`)
      assert.equal(form.combos().length, 1)
      if (field === 'endpoint') {
        assert.equal(form.button('获取模型').props.disabled, true, 'stored key cannot go to a changed destination')
      }
    } finally { form.dispose() }
  }
})

test('empty or failed listings keep manual model entry available', async () => {
  for (const result of [
    { ...success, models: [] },
    { ok: false, models: [], errorCode: 'unavailable', message: '' },
  ]) {
    const form = mountForm(async () => result)
    try {
      await event(form.button('获取模型'), 'onClick')
      await settle()
      const combo = form.combos()[0]!
      event(combo, 'onFocus')
      event(combo, 'onInput', { target: { value: 'fallback-model' } })
      await event(form.find((element) => element.kind === 'form'), 'onSubmit', { preventDefault() {} })
      assert.deepEqual(form.submitted()?.models, ['fallback-model'])
    } finally { form.dispose() }
  }
})

test('source edits clear completed candidates and newer fetches win over late failures', async () => {
  let rejectOld!: (error: Error) => void
  let calls = 0
  const form = mountForm(() => {
    calls++
    if (calls === 1) return Promise.resolve(success)
    if (calls === 2) return new Promise((_resolve, reject) => { rejectOld = reject })
    return Promise.resolve({ ...success, models: ['new-source-model'] })
  })
  try {
    await event(form.button('获取模型'), 'onClick')
    await settle()
    const secret = form.find((element) => element.kind === 'input' && element.props.type === 'password')
    event(secret, 'onUpdate:modelValue', 'fixture-new-key')
    await settle()
    event(form.combos()[0]!, 'onFocus')
    await settle()
    assert.deepEqual(form.options(), [], 'completed candidates belong to the previous key')
    const old = event(form.button('获取模型'), 'onClick')
    event(form.button('获取模型'), 'onClick')
    assert.equal(calls, 2, 'duplicate clicks cannot create concurrent requests for the same source')
    event(secret, 'onUpdate:modelValue', 'fixture-newer-key')
    await event(form.button('获取模型'), 'onClick')
    await settle()
    rejectOld(new Error('anonymous failure'))
    await old
    await settle()
    assert.deepEqual(form.options().map(content), ['new-source-model'])
  } finally { form.dispose() }
})
