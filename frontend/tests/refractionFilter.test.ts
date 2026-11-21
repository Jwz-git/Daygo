import assert from 'node:assert/strict'
import test from 'node:test'

import { installRefractionFilter, setRefractionIntensity } from '@/lib/refractionFilter'

test('changing strength replaces the SVG and disabling removes it across remounts', () => {
  const children: SvgNode[] = []
  class SvgNode {
    id = ''
    style = {}
    setAttribute(name: string, value: string) { if (name === 'id') this.id = value }
    append() {}
    remove() {
      const index = children.indexOf(this)
      if (index !== -1) children.splice(index, 1)
    }
  }

  const previous = globalThis.document
  globalThis.document = {
    body: { append: (node: SvgNode) => children.push(node) },
    documentElement: { dataset: {} },
    createElementNS: () => new SvgNode(),
    getElementById: (id: string) => children.find((node) => node.id === id) ?? null,
  } as unknown as Document

  try {
    installRefractionFilter('subtle')
    const first = children[0]
    assert.equal(children.length, 1)
    setRefractionIntensity('normal')
    assert.equal(children.length, 1)
    assert.notEqual(children[0], first)
    setRefractionIntensity('off')
    assert.equal(children.length, 0)
    installRefractionFilter()
    assert.equal(children.length, 1)
    setRefractionIntensity('off')
    assert.equal(children.length, 0)
  } finally {
    setRefractionIntensity('off')
    globalThis.document = previous
  }
})
