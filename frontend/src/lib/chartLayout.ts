/*
 * Geometry for the weekly treemap, kept free of Vue so it can be tested
 * directly. Mirrors Dayflow's SquarifiedTreemapLayout (Views/UI/Weekly/Sections,
 * MIT). The sankey's geometry lives in lib/sankeyLayout.
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
