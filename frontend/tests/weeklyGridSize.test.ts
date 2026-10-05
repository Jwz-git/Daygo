import assert from 'node:assert/strict'
import test from 'node:test'
import { createRenderer, nextTick, type Component } from 'vue'
import { createI18n } from 'vue-i18n'

import zhCN from '../src/locales/zh-CN'
import WeeklyHeatmapCard from '../src/views/Weekly/charts/WeeklyHeatmapCard.vue'
import WeeklyWorkflowCard from '../src/views/Weekly/charts/WeeklyWorkflowCard.vue'

interface HostNode {
  kind: string
  children: HostNode[]
  parent: HostNode | null
  props: Record<string, unknown>
  text: string
  clientWidth: number
  getBoundingClientRect: () => DOMRect
  classList: { add: () => void; remove: () => void }
}

function collect(root: HostNode): HostNode[] {
  return [root, ...root.children.flatMap(collect)]
}

function mountChart(component: Component, snapshot: unknown, initialWidth: number) {
  const previous = {
    ResizeObserver: globalThis.ResizeObserver,
    SVGSVGElement: globalThis.SVGSVGElement,
    DOMPoint: globalThis.DOMPoint,
    requestAnimationFrame: globalThis.requestAnimationFrame,
    window: globalThis.window,
    document: globalThis.document,
  }
  let resized: ResizeObserverCallback | undefined
  let observed: Element | undefined
  let disconnected = false
  globalThis.ResizeObserver = class {
    constructor(callback: ResizeObserverCallback) { resized = callback }
    observe(element: Element) { observed = element }
    disconnect() { disconnected = true }
    unobserve() {}
  } as unknown as typeof ResizeObserver
  class FixtureSVG {
    getScreenCTM() { return { inverse() { return {} } } }
  }
  globalThis.SVGSVGElement = FixtureSVG as unknown as typeof SVGSVGElement
  globalThis.DOMPoint = class {
    constructor(readonly x: number, readonly y: number) {}
    matrixTransform() { return { x: this.x, y: this.y } }
  } as unknown as typeof DOMPoint
  // The host fixture has no CSS animation; complete tooltip transitions
  // immediately while retaining the real component and pointer handlers.
  globalThis.requestAnimationFrame = (callback) => { callback(0); return 1 }
  globalThis.window = {
    getComputedStyle: () => ({ transitionDelay: '0s', transitionDuration: '0s', animationDelay: '0s', animationDuration: '0s' }),
  } as unknown as Window & typeof globalThis
  globalThis.document = { body: { offsetHeight: 0 } } as unknown as Document

  function node(kind: string, text = ''): HostNode {
    return {
      kind, text, children: [], parent: null, props: {}, clientWidth: initialWidth,
      getBoundingClientRect: () => ({ left: 0, top: 0, width: initialWidth } as DOMRect),
      classList: { add() {}, remove() {} },
    }
  }
  const renderer = createRenderer<HostNode, HostNode>({
    createElement: node, createText: (text) => node('#text', text),
    createComment: (text) => node('#comment', text),
    insert(child, parent, anchor = null) {
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
  })
  const root = node('root')
  const days = Array.from({ length: 7 }, (_, index) => `2026-10-${String(5 + index).padStart(2, '0')}`)
  const app = renderer.createApp(component, { snapshot, days })
  app.use(createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN } }))
  app.mount(root)
  return {
    root,
    async hover(svg: HostNode, x: number, y: number) {
      const move = svg.props.onPointermove as (event: unknown) => void
      move({ currentTarget: new FixtureSVG(), clientX: x, clientY: y })
      await nextTick()
    },
    async leave(svg: HostNode) {
      const leave = svg.props.onPointerleave as () => void
      leave()
      await nextTick()
    },
    async resize(width: number) {
      assert.ok(observed && resized, 'the grid observes its available width')
      const entry = { target: observed, contentRect: { width } } as ResizeObserverEntry
      resized([entry], {} as ResizeObserver)
      await nextTick()
    },
    dispose() {
      app.unmount()
      Object.assign(globalThis, previous)
      return disconnected
    },
  }
}

const cases = [
  { name: 'workflow', component: WeeklyWorkflowCard, bucket: 15, cellHeight: 13, height: 127, labelWidth: 40 },
  { name: 'heatmap', component: WeeklyHeatmapCard, bucket: 5, cellHeight: 12, height: 118, labelWidth: 36 },
]

for (const chart of cases) {
  test(`${chart.name} keeps text and row geometry at 1:1 for short and full-day windows`, async () => {
    // A new week with just 30 recorded minutes produces a padded 90-minute
    // window. It must have the same row height as a 13-hour or 24-hour week.
    for (const span of [90, 780, 1440]) {
      const count = span / chart.bucket
      const cell = chart.name === 'workflow'
        ? { category: 'Coding', colorHex: '#4F8CFF', minutes: 15, occupancy: 1 }
        : -1
      const snapshot = {
        start: 240, end: 240 + span, slotMinutes: chart.bucket, bucketMinutes: chart.bucket,
        rows: Array.from({ length: 7 }, () => Array.from({ length: count }, () => cell)), totals: [],
      }
      const mounted = mountChart(chart.component, snapshot, 1100)
      let disconnected = false
      try {
        await nextTick()
        for (const available of [1100, 720, 400]) {
          if (available !== 1100) await mounted.resize(available)
          const width = Math.max(560, available)
          const svg = collect(mounted.root).find((element) => element.kind === 'svg')!
          assert.equal(svg.props.width, width, 'SVG viewport follows available width, with narrow scrolling')
          assert.equal(svg.props.height, chart.height, 'short activity cannot increase chart height')
          assert.equal(svg.props.viewBox, `0 0 ${width} ${chart.height}`, '1 SVG unit equals 1 CSS pixel')
          const cells = collect(svg).filter((element) => element.kind === 'rect')
          assert.equal(cells.length, count * 7, 'all time buckets remain present')
          assert.ok(cells.every((cell) => cell.props.height === chart.cellHeight))
          assert.ok(cells.every((cell) => Number(cell.props.width) > 0))
          const lastCell = cells[count - 1]
          assert.ok(Number(lastCell.props.x) + Number(lastCell.props.width) <= width - 24)
          const ticks = svg.children.filter((element) => element.kind === 'text')
          assert.ok(ticks.every((tick) => Number(tick.props.x) >= chart.labelWidth && Number(tick.props.x) <= width - 24))
          for (let index = 1; index < ticks.length; index++) {
            assert.ok(Number(ticks[index].props.x) - Number(ticks[index - 1].props.x) >= 48, 'fixed-size hour labels have room')
          }
          // Changing horizontal bucket sizes must keep hover on the same
          // last bucket of Thursday; the existing SVG coordinate mapper is
          // represented by an identity transform in this anonymous host.
          const target = cells[4 * count - 1]
          await mounted.hover(svg, Number(target.props.x) + Number(target.props.width) / 2, Number(target.props.y) + chart.cellHeight / 2)
          const cursor = collect(svg).find((element) => String(element.props.class).includes('__cursor'))
          assert.ok(cursor, 'hover renders the selected bucket cursor')
          assert.equal((cursor.props.style as { transform: string }).transform,
            `translate(${Number(target.props.x) - 1}px, ${Number(target.props.y) - 1}px)`)
          await mounted.leave(svg)
          assert.ok(!collect(svg).some((element) => String(element.props.class).includes('__cursor')))
        }
      } finally {
        disconnected = mounted.dispose()
      }
      assert.ok(disconnected, 'the grid releases its observer on unmount')
    }
  })
}
