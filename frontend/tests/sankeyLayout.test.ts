import assert from 'node:assert/strict'
import test from 'node:test'

import { SANKEY_COLUMNS, sankeyGradientStops, sankeyLayout, sankeyRibbonPath, sankeyRibbonTint } from '../src/lib/sankeyLayout'

/*
 * Two categories feeding three apps; "Shared" is used under both, so it must
 * sit between the apps fed only by the top or only by the bottom category.
 * Expected values follow Dayflow's WeeklySankeyModelFactory.
 */
const categories = [
  { key: 'Coding', minutes: 300, colorHex: '#93BCFF' },
  { key: 'Personal', minutes: 100, colorHex: '#FFC6B7' },
]
const apps = [
  { key: 'other', minutes: 20, colorHex: '#D9D9D9' },
  { key: 'Messages', minutes: 60, colorHex: '#38D06E' },
  { key: 'Shared', minutes: 120, colorHex: '#24292F' },
  { key: 'Editor', minutes: 200, colorHex: '#6CDACD' },
]
const links = [
  { from: 'Coding', to: 'Editor', minutes: 200 },
  { from: 'Coding', to: 'Shared', minutes: 80 },
  { from: 'Coding', to: 'other', minutes: 20 },
  { from: 'Personal', to: 'Shared', minutes: 40 },
  { from: 'Personal', to: 'Messages', minutes: 60 },
]
const layout = sankeyLayout(categories, apps, links)
const close = (actual: number, expected: number): void => assert.ok(Math.abs(actual - expected) < 1e-6, `${actual} ≈ ${expected}`)

test('columns open outwards: source 433, categories and apps fill taller spans', () => {
  close(layout.source.height, 433)
  const span = (bands: typeof layout.apps) => bands.at(-1)!.y + bands.at(-1)!.height - bands[0].y
  close(span(layout.categories), SANKEY_COLUMNS.categories.bottom - SANKEY_COLUMNS.categories.top)
  close(span(layout.apps), SANKEY_COLUMNS.apps.bottom - SANKEY_COLUMNS.apps.top)
  // 702 − 20 gap = 682; each band gets 40 + 602 × share.
  close(layout.categories[0].height, 40 + 602 * 0.75)
})

test('a source ribbon spans its segment of the source and the whole category band', () => {
  const coding = layout.flows.find((flow) => flow.kind === 'source' && flow.to === 'Coding')!
  const band = layout.categories[0]
  close(coding.y0Top, 273)
  close(coding.y0Bottom, 273 + 433 * 0.75)
  close(coding.y1Top, band.y)
  close(coding.y1Bottom, band.y + band.height)
  assert.equal(coding.tension, 0.15)
})

test('apps sit at the weighted centre of their categories, other last', () => {
  assert.deepEqual(layout.apps.map((app) => app.key), ['Editor', 'Shared', 'Messages', 'other'])
})

test('link segments tile every category and app band exactly', () => {
  for (const band of [...layout.categories, ...layout.apps]) {
    const isCategory = layout.categories.includes(band)
    const touching = layout.flows.filter((flow) => flow.kind === 'link' && (isCategory ? flow.from : flow.to) === band.key)
    const edges = touching
      .map((flow) => (isCategory ? [flow.y0Top, flow.y0Bottom] : [flow.y1Top, flow.y1Bottom]))
      .sort((a, b) => a[0] - b[0])
    close(edges[0][0], band.y)
    close(edges.at(-1)![1], band.y + band.height)
    for (let index = 1; index < edges.length; index += 1) close(edges[index][0], edges[index - 1][1])
  }
  // "Shared" receives Coding above Personal, as their bands are ordered.
  const shared = layout.flows.filter((flow) => flow.kind === 'link' && flow.to === 'Shared')
  assert.ok(shared.find((flow) => flow.from === 'Coding')!.y1Top < shared.find((flow) => flow.from === 'Personal')!.y1Top)
})

test('labels never overlap and stay inside their column limits', () => {
  for (const [bands, spec] of [[layout.categories, SANKEY_COLUMNS.categories], [layout.apps, SANKEY_COLUMNS.apps]] as const) {
    const tops = bands.map((band) => band.labelY).sort((a, b) => a - b)
    assert.ok(tops[0] >= spec.labelTop)
    assert.ok(tops.at(-1)! + spec.labelHeight <= spec.labelBottom + 1e-6)
    for (let index = 1; index < tops.length; index += 1) {
      assert.ok(tops[index] - tops[index - 1] >= spec.labelHeight + spec.labelSpacing - 1e-6)
    }
  }
  // Crowded column: twelve thin apps still fit between the limits.
  const many = Array.from({ length: 12 }, (_, index) => ({ key: `a${index}`, minutes: index === 0 ? 1000 : 1, colorHex: '#93BCFF' }))
  const crowded = sankeyLayout(categories, many, many.map((app) => ({ from: 'Coding', to: app.key, minutes: app.minutes })))
  const tops = crowded.apps.map((band) => band.labelY).sort((a, b) => a - b)
  assert.ok(tops.at(-1)! + SANKEY_COLUMNS.apps.labelHeight <= SANKEY_COLUMNS.apps.labelBottom + 1e-6)
})

test('ribbon curves reach at least 90 units and tint dark bars warm grey', () => {
  assert.equal(
    sankeyRibbonPath(0, 0, 10, 100, 0, 10, 0.15),
    'M0.00 0.00 C90.00 0.00 10.00 0.00 100.00 0.00 L100.00 10.00 C10.00 10.00 90.00 10.00 0.00 10.00 Z',
  )
  assert.equal(sankeyRibbonTint('#000000'), '#CAC2BA')
  assert.equal(sankeyRibbonTint('#bfb6ae'), '#CFC8C1')
  const stops = sankeyGradientStops(layout.flows.find((flow) => flow.kind === 'link')!)
  assert.equal(stops.length, 5)
  assert.ok(stops.every((stop) => stop.opacity <= 0.27))
})
