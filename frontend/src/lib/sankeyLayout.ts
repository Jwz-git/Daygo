/*
 * Weekly sankey geometry, ported from Dayflow's WeeklySankeyModelFactory and
 * WeeklySankeyComponents (Views/UI/Weekly/Sections, MIT, Copyright (c) 2025
 * Jerry Liu).
 *
 * Everything is laid out in Dayflow's 1748×933 virtual space. The columns
 * grow from left to right (source 433 tall, categories 702, apps 874), and a
 * ribbon always spans its whole target band, so the chart opens outwards
 * instead of conserving height. Apps sit at the weighted centre of the
 * categories that feed them, with "other" last.
 */

export const SANKEY_WIDTH = 1748
export const SANKEY_HEIGHT = 933
export const SANKEY_BAR_WIDTH = 12
const SOURCE_TENSION = 0.15
const LINK_TENSION = 0.42
const OTHER_KEY = 'other'
const SOURCE_COLOR = '#D9CBC0'

interface ColumnSpec {
  x: number
  top: number
  bottom: number
  gap: number
  minHeight: number
  labelX: number
  labelTop: number
  labelBottom: number
  labelHeight: number
  labelSpacing: number
}

export const SANKEY_COLUMNS = {
  source: { x: 72, top: 273, bottom: 706, gap: 0, minHeight: 0, labelX: 105, labelTop: 0, labelBottom: 0, labelHeight: 52, labelSpacing: 0 },
  categories: { x: 760, top: 126, bottom: 828, gap: 20, minHeight: 40, labelX: 802, labelTop: 64, labelBottom: 874, labelHeight: 54, labelSpacing: 12 },
  apps: { x: 1334, top: 54, bottom: 928, gap: 20, minHeight: 28, labelX: 1372, labelTop: 38, labelBottom: 923, labelHeight: 56, labelSpacing: 10 },
} satisfies Record<string, ColumnSpec>

export interface SankeyInputNode {
  key: string
  minutes: number
  colorHex: string
}

export interface SankeyInputLink {
  from: string
  to: string
  minutes: number
}

export interface SankeyBand {
  key: string
  minutes: number
  colorHex: string
  x: number
  y: number
  height: number
  /** Top of the node's label block. */
  labelY: number
}

export interface SankeyFlow {
  /** Positional id, safe inside url(#…). */
  id: string
  kind: 'source' | 'link'
  /** Category key for source flows' target and link flows' source. */
  from: string
  to: string
  minutes: number
  fromColorHex: string
  toColorHex: string
  x0: number
  y0Top: number
  y0Bottom: number
  x1: number
  y1Top: number
  y1Bottom: number
  tension: number
  /** Dayflow's per-flow strength, which scales the gradient stops. */
  opacity: number
}

export interface SankeyLayout {
  source: SankeyBand
  categories: SankeyBand[]
  apps: SankeyBand[]
  flows: SankeyFlow[]
}

function allocateBands(items: SankeyInputNode[], spec: ColumnSpec): SankeyBand[] {
  if (items.length === 0) return []
  const available = Math.max(
    items.length * spec.minHeight,
    spec.bottom - spec.top - spec.gap * Math.max(0, items.length - 1),
  )
  const total = items.reduce((sum, item) => sum + item.minutes, 0)
  const flexible = Math.max(0, available - spec.minHeight * items.length)
  let cursor = spec.top
  return items.map((item) => {
    const height = total > 0
      ? spec.minHeight + (flexible * item.minutes) / total
      : spec.minHeight + flexible / items.length
    const band = { key: item.key, minutes: item.minutes, colorHex: item.colorHex, x: spec.x, y: cursor, height, labelY: 0 }
    cursor += height + spec.gap
    return band
  })
}

function stackSegments<T extends { minutes: number }>(items: T[], top: number, height: number): Array<T & { top: number; bottom: number }> {
  const total = items.reduce((sum, item) => sum + item.minutes, 0)
  let cursor = top
  return items.map((item) => {
    const size = total > 0 ? (item.minutes / total) * height : height / items.length
    const segment = { ...item, top: cursor, bottom: cursor + size }
    cursor += size
    return segment
  })
}

// Labels sit centred on their bar, pushed down to avoid overlap, then pulled
// back up from the bottom limit if the column overflows (Dayflow placeLabels).
function placeLabels(bands: SankeyBand[], spec: ColumnSpec): void {
  const sorted = [...bands].sort((a, b) => a.y + a.height / 2 - (b.y + b.height / 2))
  let cursor = spec.labelTop
  for (const band of sorted) {
    band.labelY = Math.max(band.y + band.height / 2 - spec.labelHeight / 2, cursor)
    cursor = band.labelY + spec.labelHeight + spec.labelSpacing
  }
  const last = sorted.length - 1
  if (last < 0) return
  const overflow = sorted[last].labelY + spec.labelHeight - spec.labelBottom
  if (overflow <= 0) return
  sorted[last].labelY -= overflow
  for (let index = last - 1; index >= 0; index -= 1) {
    sorted[index].labelY = Math.min(sorted[index].labelY, sorted[index + 1].labelY - spec.labelHeight - spec.labelSpacing)
  }
  if (sorted[0].labelY < spec.labelTop) {
    sorted[0].labelY = spec.labelTop
    for (let index = 1; index <= last; index += 1) {
      sorted[index].labelY = Math.max(sorted[index].labelY, sorted[index - 1].labelY + spec.labelHeight + spec.labelSpacing)
    }
  }
}

export function sankeyLayout(
  categories: SankeyInputNode[],
  apps: SankeyInputNode[],
  links: SankeyInputLink[],
): SankeyLayout {
  const { source: sourceSpec, categories: categorySpec, apps: appSpec } = SANKEY_COLUMNS
  const totalMinutes = categories.reduce((sum, node) => sum + node.minutes, 0)
  const categoryBands = allocateBands(categories, categorySpec)
  const categoryByKey = new Map(categoryBands.map((band) => [band.key, band]))

  const barycenter = (key: string): number => {
    let weighted = 0
    let total = 0
    for (const link of links) {
      if (link.to !== key) continue
      const category = categoryByKey.get(link.from)
      if (category === undefined) continue
      weighted += (category.y + category.height / 2) * link.minutes
      total += link.minutes
    }
    return total > 0 ? weighted / total : 999
  }
  const orderedApps = apps
    .map((app, index) => ({ app, index, center: barycenter(app.key) }))
    .sort((a, b) => {
      if (a.app.key === OTHER_KEY) return 1
      if (b.app.key === OTHER_KEY) return -1
      return a.center - b.center || a.index - b.index
    })
    .map((entry) => entry.app)
  const appBands = allocateBands(orderedApps, appSpec)
  const appByKey = new Map(appBands.map((band) => [band.key, band]))

  const source: SankeyBand = {
    key: 'source',
    minutes: totalMinutes,
    colorHex: SOURCE_COLOR,
    x: sourceSpec.x,
    y: sourceSpec.top,
    height: sourceSpec.bottom - sourceSpec.top,
    labelY: (sourceSpec.top + sourceSpec.bottom) / 2 - sourceSpec.labelHeight / 2,
  }

  const flows: SankeyFlow[] = []
  for (const segment of stackSegments(categoryBands, source.y, source.height)) {
    flows.push({
      id: `f${flows.length}`,
      kind: 'source',
      from: segment.key,
      to: segment.key,
      minutes: segment.minutes,
      fromColorHex: SOURCE_COLOR,
      toColorHex: segment.colorHex,
      x0: source.x + SANKEY_BAR_WIDTH,
      y0Top: segment.top,
      y0Bottom: segment.bottom,
      x1: segment.x,
      y1Top: segment.y,
      y1Bottom: segment.y + segment.height,
      tension: SOURCE_TENSION,
      opacity: 0.14 + 0.08 * Math.sqrt(segment.minutes / Math.max(totalMinutes, 1)),
    })
  }

  const visibleLinks = links.filter((link) => categoryByKey.has(link.from) && appByKey.has(link.to))
  const outgoingTop = new Map<string, { top: number; bottom: number }>()
  for (const category of categoryBands) {
    const outgoing = visibleLinks
      .filter((link) => link.from === category.key)
      .sort((a, b) => appByKey.get(a.to)!.y - appByKey.get(b.to)!.y)
    for (const segment of stackSegments(outgoing, category.y, category.height)) {
      outgoingTop.set(`${segment.from}\u0000${segment.to}`, segment)
    }
  }
  const incomingTop = new Map<string, { top: number; bottom: number }>()
  for (const app of appBands) {
    const incoming = visibleLinks
      .filter((link) => link.to === app.key)
      .sort((a, b) => categoryByKey.get(a.from)!.y - categoryByKey.get(b.from)!.y)
    for (const segment of stackSegments(incoming, app.y, app.height)) {
      incomingTop.set(`${segment.from}\u0000${segment.to}`, segment)
    }
  }

  const maxLinkMinutes = Math.max(1, ...visibleLinks.map((link) => link.minutes))
  for (const link of visibleLinks) {
    const category = categoryByKey.get(link.from)!
    const app = appByKey.get(link.to)!
    const out = outgoingTop.get(`${link.from}\u0000${link.to}`)!
    const into = incomingTop.get(`${link.from}\u0000${link.to}`)!
    flows.push({
      id: `f${flows.length}`,
      kind: 'link',
      from: link.from,
      to: link.to,
      minutes: link.minutes,
      fromColorHex: category.colorHex,
      toColorHex: app.colorHex,
      x0: category.x + SANKEY_BAR_WIDTH,
      y0Top: out.top,
      y0Bottom: out.bottom,
      x1: app.x,
      y1Top: into.top,
      y1Bottom: into.bottom,
      tension: LINK_TENSION,
      opacity: 0.08 + 0.18 * Math.sqrt(link.minutes / maxLinkMinutes),
    })
  }

  placeLabels(categoryBands, categorySpec)
  placeLabels(appBands, appSpec)
  return { source, categories: categoryBands, apps: appBands, flows }
}

/** Cubic ribbon between two vertical edges; the curve reach is at least 90 units. */
export function sankeyRibbonPath(
  x0: number, y0Top: number, y0Bottom: number,
  x1: number, y1Top: number, y1Bottom: number,
  tension: number,
): string {
  const curve = Math.max(90, (x1 - x0) * tension)
  const f = (value: number): string => value.toFixed(2)
  return `M${f(x0)} ${f(y0Top)} C${f(x0 + curve)} ${f(y0Top)} ${f(x1 - curve)} ${f(y1Top)} ${f(x1)} ${f(y1Top)} `
    + `L${f(x1)} ${f(y1Bottom)} C${f(x1 - curve)} ${f(y1Bottom)} ${f(x0 + curve)} ${f(y0Bottom)} ${f(x0)} ${f(y0Bottom)} Z`
}

/** Dark and neutral bars tint their ribbons with a warm grey, as in Dayflow. */
export function sankeyRibbonTint(colorHex: string): string {
  const normalized = colorHex.replace('#', '').toUpperCase()
  if (normalized === '000000' || normalized === '333333') return '#CAC2BA'
  if (normalized === 'D9D9D9' || normalized === 'BFB6AE') return '#CFC8C1'
  return `#${normalized}`
}

export interface SankeyStop {
  offset: number
  color: string
  opacity: number
}

/** Dayflow's gradient stops: source flows warm-neutral into the category, links category into app. */
export function sankeyGradientStops(flow: SankeyFlow): SankeyStop[] {
  const strength = Math.max(0.08, Math.min(flow.opacity, 0.36))
  const from = sankeyRibbonTint(flow.fromColorHex)
  const to = sankeyRibbonTint(flow.toColorHex)
  if (flow.kind === 'source') {
    return [
      { offset: 0, color: '#E3D8CF', opacity: 0.18 },
      { offset: 0.24, color: '#ECE3DC', opacity: 0.16 },
      { offset: 0.58, color: to, opacity: Math.min(0.12, strength * 0.42) },
      { offset: 0.82, color: to, opacity: Math.min(0.2, strength * 0.72) },
      { offset: 1, color: to, opacity: Math.min(0.32, strength * 1.08) },
    ]
  }
  return [
    { offset: 0, color: from, opacity: Math.min(0.2, strength * 0.68) },
    { offset: 0.24, color: from, opacity: Math.min(0.11, strength * 0.4) },
    { offset: 0.54, color: to, opacity: Math.min(0.05, strength * 0.2) },
    { offset: 0.78, color: to, opacity: Math.min(0.12, strength * 0.42) },
    { offset: 1, color: to, opacity: Math.min(0.27, strength * 0.9) },
  ]
}
