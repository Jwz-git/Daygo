import assert from 'node:assert/strict'
import test from 'node:test'
import { createRenderer, defineComponent, h, nextTick, onMounted, ref } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import CardVideoPlayer from '../src/components/CardVideoPlayer.vue'
import { useUIVisibilityStore } from '../src/stores/uiVisibility'

// Vue host renderer: exercise the real SFC without adding a DOM dependency.
interface HostNode {
  kind: string
  children: HostNode[]
  parent: HostNode | null
  props: Record<string, unknown>
  text: string
  classList: { add: (...values: string[]) => void; remove: (...values: string[]) => void }
  addEventListener: () => void
  removeEventListener: () => void
  ownerDocument: { defaultView: typeof globalThis }
}
function node(kind: string): HostNode {
  return { kind, children: [], parent: null, props: {}, text: '',
    classList: { add() {}, remove() {} }, addEventListener() {}, removeEventListener() {},
    ownerDocument: { defaultView: globalThis } }
}

test('real player removes hidden images while retaining rate, progress and playback intention', async () => {
  const previous = { window: globalThis.window, document: globalThis.document, Image: globalThis.Image,
    request: globalThis.requestAnimationFrame, cancel: globalThis.cancelAnimationFrame,
    style: globalThis.getComputedStyle }
  const root = node('root')
  const body = node('body')
  let nativeEvent: ((...payload: unknown[]) => void) | undefined
  let raf = 0
  const callbacks = new Map<number, FrameRequestCallback>()
  const warmed: { src: string }[] = []
  globalThis.window = Object.assign(new EventTarget(), { go: { app: { Backend: {
    GetUIVisibility: async () => ({ visible: true }),
  } } }, runtime: { EventsOnMultiple: (_name: string, callback: (...payload: unknown[]) => void) => {
    nativeEvent = callback
    return () => { nativeEvent = undefined }
  } } }) as unknown as Window & typeof globalThis
  globalThis.document = Object.assign(new EventTarget(), { visibilityState: 'visible', body }) as unknown as Document
  globalThis.Image = class {
    src = ''; onload = null; onerror = null
    constructor() { warmed.push(this) }
    removeAttribute(): void { this.src = '' }
  } as unknown as typeof Image
  globalThis.requestAnimationFrame = (callback) => { callbacks.set(++raf, callback); return raf }
  globalThis.cancelAnimationFrame = (id) => { callbacks.delete(id) }
  globalThis.getComputedStyle = () => ({ transitionDelay: '0s', transitionDuration: '0s',
    animationDelay: '0s', animationDuration: '0s' }) as CSSStyleDeclaration
  const renderer = createRenderer<HostNode, HostNode>({
    createElement: (kind) => node(kind), createText: (text) => Object.assign(node('text'), { text }),
    createComment: (text) => Object.assign(node('comment'), { text }),
    insert(child, parent, anchor = null) {
      if (child.parent) { const old = child.parent.children; old.splice(old.indexOf(child), 1) }
      child.parent = parent
      const index = anchor ? parent.children.indexOf(anchor) : -1
      if (index < 0) parent.children.push(child); else parent.children.splice(index, 0, child)
    },
    remove(child) { if (child.parent) { const children = child.parent.children; children.splice(children.indexOf(child), 1); child.parent = null } },
    setText: (element, text) => { element.text = text },
    setElementText: (element, text) => { element.text = text; element.children = [] },
    parentNode: (element) => element.parent,
    nextSibling: (element) => element.parent?.children[(element.parent.children.indexOf(element)) + 1] ?? null,
    patchProp: (element, key, _old, value) => { element.props[key] = value },
    querySelector: () => body,
  })
  const collect = (kind: string, where = root): HostNode[] => [
    ...(where.kind === kind ? [where] : []), ...where.children.flatMap((child) => collect(kind, child)),
  ]
  const find = (className: string): HostNode => {
    const found = collect('div').concat(collect('button')).find((element) => element.props.class === className)
    assert.ok(found, `missing ${className}`)
    return found
  }
  const playerProps = { frames: [
    { id: 1, capturedAt: 1000 }, { id: 2, capturedAt: 1100 }, { id: 3, capturedAt: 1200 }, { id: 4, capturedAt: 1300 },
  ], title: 'Anonymous', timeLabel: '', timeZone: 'UTC' }
  let mounts = 0
  const Parent = defineComponent({ setup() {
    const draft = ref('unsaved anonymous draft')
    onMounted(() => { mounts += 1 })
    return () => h('section', [h('input', { value: draft.value, onInput: (event: { target: { value: string } }) => { draft.value = event.target.value } }), h(CardVideoPlayer, playerProps)])
  } })
  const app = renderer.createApp(Parent)
  const pinia = createPinia()
  app.use(pinia).use(createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': {
    timeline: { player: { play: '播放', pause: '暂停', rate: '倍速', expand: '展开', close: '关闭' } },
  } } }))
  const visibility = useUIVisibilityStore(pinia)
  try {
    visibility.start()
    app.mount(root)
    await nextTick()
    assert.equal(collect('img').length, 1)
    ;(collect('input')[0]!.props.onInput as (event: unknown) => void)({ target: { value: 'edited anonymous draft' } })
    await nextTick()
    assert.equal(warmed.length, 0, 'cover alone should not warm full-size frames')
    ;(find('player__rate').props.onClick as (event: unknown) => void)({ stopPropagation() {} })
    ;(find('player__big-play').props.onClick as (event: unknown) => void)({ stopPropagation() {} })
    const tick = async (now: number): Promise<void> => {
      const pending = [...callbacks.values()]; callbacks.clear()
      for (const callback of pending) callback(now)
      await nextTick()
    }
    await tick(1000); await tick(2000)
    const progress = find('player__bar').props['aria-valuenow']
    const rate = find('player__rate').text
    const count = warmed.length
    nativeEvent?.({ visible: false })
    await nextTick()
    assert.equal(collect('img').length, 0)
    assert.equal(callbacks.size, 0)
    assert.ok(warmed.every((image) => image.src === ''))
    assert.equal(warmed.length, count)
    assert.equal(find('player__bar').props['aria-valuenow'], progress)
    nativeEvent?.({ visible: true })
    await nextTick()
    assert.equal(collect('img').length, 1)
    assert.equal(mounts, 1)
    assert.equal(collect('input')[0]?.props.value, 'edited anonymous draft')
    assert.equal(find('player__rate').text, rate)
    await tick(100000)
    assert.equal(find('player__bar').props['aria-valuenow'], progress, 'hidden time must not advance the clock')
    await tick(101000)
    assert.ok(Number(find('player__bar').props['aria-valuenow']) > Number(progress))
    ;(collect('img')[0]!.props.onClick as () => void)()
    await nextTick()
    nativeEvent?.({ visible: false })
    await nextTick()
    nativeEvent?.({ visible: true })
    await nextTick()
    assert.equal(callbacks.size, 0, 'paused player must stay paused on reopen')
    assert.equal(mounts, 1)
    assert.equal(collect('input')[0]?.props.value, 'edited anonymous draft')
    ;(window as unknown as { getComputedStyle: typeof getComputedStyle }).getComputedStyle = getComputedStyle
    ;(find('player__expand').props.onClick as (event: unknown) => void)({ stopPropagation() {} })
    await nextTick()
    await tick(200000); await tick(200001)
    assert.equal(collect('img', body).length, 5)
    assert.ok(collect('img', body).filter((image) => image.props.class === 'filmstrip__thumb')
      .every((image) => String(image.props.src).startsWith('/media/thumbnail?id=')))
    nativeEvent?.({ visible: false })
    await nextTick()
    assert.equal(collect('img', body).length, 0, 'hidden lightbox must release images without waiting for an animation')
    nativeEvent?.({ visible: true })
    await nextTick()
    assert.equal(collect('img', body).length, 5, 'expanded state must survive hide/show')
    assert.equal(callbacks.size, 0)
  } finally {
    app.unmount()
    visibility.stop()
    Object.assign(globalThis, { window: previous.window, document: previous.document, Image: previous.Image,
      requestAnimationFrame: previous.request, cancelAnimationFrame: previous.cancel, getComputedStyle: previous.style })
  }
})
