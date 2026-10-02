/*
 * Geometry for the weekly treemap and sankey, kept free of Vue so it can be
 * tested directly. Mirrors Dayflow's SquarifiedTreemapLayout and its sankey
 * column stacking (Views/UI/Weekly/Sections, MIT).
 */

export interface Rect {
  x: number
  y: number
  width: number
  height: number
}

export interface Placement<T> {
  item: T
  rect: Rect
}

function worstRatio(row: number[], side: number): number {
  const sum = row.reduce((total, value) => total + value, 0)
  if (sum <= 0 || side <= 0) return Number.POSITIVE_INFINITY
  const max = Math.max(...row)
  const min = Math.min(...row)
  const sideSquared = side * side
  const sumSquared = sum * sum
  return Math.max((sideSquared * max) / sumSquared, sumSquared / (sideSquared * min))
}

/**
 * Squarified treemap (Bruls, Huizing, van Wijk). Items are laid out in the
 * given order, biggest first, so tiles keep a stable reading order. `gap` is
 * subtracted from each tile's trailing edges, so adjacent tiles never touch.
 */
export function squarify<T>(items: T[], value: (item: T) => number, bounds: Rect, gap = 0): Placement<T>[] {
  const entries = items
    .map((item) => ({ item, value: Math.max(0, value(item)) }))
    .filter((entry) => entry.value > 0)
  const total = entries.reduce((sum, entry) => sum + entry.value, 0)
  if (total <= 0 || bounds.width <= 0 || bounds.height <= 0) return []
  const scale = (bounds.width * bounds.height) / total
  const areas = entries.map((entry) => entry.value * scale)

  const placements: Placement<T>[] = []
  let remaining: Rect = { ...bounds }
  let index = 0
  while (index < entries.length) {
    const side = Math.min(remaining.width, remaining.height)
    const row: number[] = [areas[index]]
    let next = index + 1
    while (next < entries.length && worstRatio([...row, areas[next]], side) <= worstRatio(row, side)) {
      row.push(areas[next])
      next += 1
    }
    const rowArea = row.reduce((sum, area) => sum + area, 0)
    const horizontal = remaining.width >= remaining.height
    const thickness = horizontal ? rowArea / remaining.height : rowArea / remaining.width
    let cursor = horizontal ? remaining.y : remaining.x
    row.forEach((area, offset) => {
      const length = area / thickness
      const rect: Rect = horizontal
        ? { x: remaining.x, y: cursor, width: thickness, height: length }
        : { x: cursor, y: remaining.y, width: length, height: thickness }
      placements.push({
        item: entries[index + offset].item,
        rect: {
          x: rect.x,
          y: rect.y,
          width: Math.max(0, rect.width - gap),
          height: Math.max(0, rect.height - gap),
        },
      })
      cursor += length
    })
    remaining = horizontal
      ? { x: remaining.x + thickness, y: remaining.y, width: remaining.width - thickness, height: remaining.height }
      : { x: remaining.x, y: remaining.y + thickness, width: remaining.width, height: remaining.height - thickness }
    index = next
  }
  // The trailing gap is only between tiles: give the outermost edge back.
  const right = bounds.x + bounds.width
  const bottom = bounds.y + bounds.height
  for (const placement of placements) {
    if (Math.abs(placement.rect.x + placement.rect.width + gap - right) < 0.5) placement.rect.width += gap
    if (Math.abs(placement.rect.y + placement.rect.height + gap - bottom) < 0.5) placement.rect.height += gap
  }
  return placements
}

export interface StackedNode {
  key: string
  y: number
  height: number
}

/**
 * Stacks nodes in one sankey column: heights proportional to minutes within
 * [top, bottom], separated by `gap`, each at least `minHeight` tall.
 */
export function stackColumn(
  nodes: Array<{ key: string; minutes: number }>,
  top: number,
  bottom: number,
  gap: number,
  minHeight = 2,
): StackedNode[] {
  const total = nodes.reduce((sum, node) => sum + Math.max(0, node.minutes), 0)
  const usable = Math.max(0, bottom - top - gap * Math.max(0, nodes.length - 1))
  let cursor = top
  return nodes.map((node) => {
    const height = total > 0 ? Math.max(minHeight, (Math.max(0, node.minutes) / total) * usable) : minHeight
    const placed = { key: node.key, y: cursor, height }
    cursor += height + gap
    return placed
  })
}

/** Closed path of a ribbon from a band on the left column to one on the right. */
export function ribbonPath(x0: number, y0: number, h0: number, x1: number, y1: number, h1: number): string {
  const mid = (x0 + x1) / 2
  return [
    `M${x0},${y0}`,
    `C${mid},${y0} ${mid},${y1} ${x1},${y1}`,
    `L${x1},${y1 + h1}`,
    `C${mid},${y1 + h1} ${mid},${y0 + h0} ${x0},${y0 + h0}`,
    'Z',
  ].join(' ')
}
