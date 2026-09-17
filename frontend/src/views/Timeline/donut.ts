export interface DonutSlice {
  label: string
  minutes: number
  color: string
}

export interface DonutSector {
  color: string
  path: string
}

/**
 * Annular-sector path for one slice (Dayflow's SectorMark): inner radius 75%
 * of the outer, a 2-degree angular inset per side forming the visible gap,
 * and corners rounded by stroking the same-colour path (round linejoin).
 */
export function sectorPath(
  cx: number,
  cy: number,
  rOuter: number,
  rInner: number,
  startAngle: number,
  endAngle: number,
): string {
  const largeArc = endAngle - startAngle > Math.PI ? 1 : 0
  const point = (radius: number, angle: number): string =>
    `${(cx + radius * Math.cos(angle)).toFixed(2)} ${(cy + radius * Math.sin(angle)).toFixed(2)}`
  return [
    `M ${point(rOuter, startAngle)}`,
    `A ${rOuter} ${rOuter} 0 ${largeArc} 1 ${point(rOuter, endAngle)}`,
    `L ${point(rInner, endAngle)}`,
    `A ${rInner} ${rInner} 0 ${largeArc} 0 ${point(rInner, startAngle)}`,
    'Z',
  ].join(' ')
}

/**
 * Renders a full 360-degree annulus (single category occupying 100% of recorded time).
 * In SVG, an arc command (A) treats coincident start and end points as a zero-length arc,
 * so a single 360-degree arc will not draw anything. We draw two 180-degree semicircles
 * for the outer circle (clockwise) and two for the inner circle (counter-clockwise).
 */
export function fullRingPath(
  cx: number,
  cy: number,
  rOuter: number,
  rInner: number,
): string {
  const pOuterTop = `${cx.toFixed(2)} ${(cy - rOuter).toFixed(2)}`
  const pOuterBottom = `${cx.toFixed(2)} ${(cy + rOuter).toFixed(2)}`
  const pInnerTop = `${cx.toFixed(2)} ${(cy - rInner).toFixed(2)}`
  const pInnerBottom = `${cx.toFixed(2)} ${(cy + rInner).toFixed(2)}`

  return [
    `M ${pOuterTop}`,
    `A ${rOuter} ${rOuter} 0 1 1 ${pOuterBottom}`,
    `A ${rOuter} ${rOuter} 0 1 1 ${pOuterTop}`,
    'Z',
    `M ${pInnerTop}`,
    `A ${rInner} ${rInner} 0 1 0 ${pInnerBottom}`,
    `A ${rInner} ${rInner} 0 1 0 ${pInnerTop}`,
    'Z',
  ].join(' ')
}

/**
 * Builds the SVG sectors for the donut chart based on total recorded time.
 * If only one category exists, it fills the complete 360-degree ring without gaps.
 * If multiple categories exist, each slice takes an angular portion proportional
 * to its share of the total recorded time, separated by the visual gap angle.
 */
export function buildDonutSectors(
  slices: DonutSlice[],
  outerRadius = 102.5,
  innerRadius = outerRadius * 0.75,
  gapAngle = (2 * Math.PI) / 180,
): DonutSector[] {
  const activeSlices = slices.filter((slice) => Number.isFinite(slice.minutes) && slice.minutes > 0)
  if (activeSlices.length === 0) {
    return []
  }

  // When only one category has recorded time, it fills the entire ring.
  if (activeSlices.length === 1) {
    return [
      {
        color: activeSlices[0].color,
        path: fullRingPath(102.5, 102.5, outerRadius, innerRadius),
      },
    ]
  }

  const total = activeSlices.reduce((sum, slice) => sum + slice.minutes, 0)
  if (total <= 0) {
    return []
  }

  let consumed = 0
  const sectors: DonutSector[] = []
  for (const slice of activeSlices) {
    const fraction = slice.minutes / total
    if (fraction <= 0) continue

    const startAngle = -Math.PI / 2 + consumed * 2 * Math.PI + gapAngle / 2
    const endAngle = -Math.PI / 2 + (consumed + fraction) * 2 * Math.PI - gapAngle / 2

    if (endAngle - startAngle < 0.008) {
      // Degenerate sliver: still draw a minimal wedge so the category shows.
      sectors.push({
        color: slice.color,
        path: sectorPath(102.5, 102.5, outerRadius, innerRadius, startAngle, startAngle + 0.008),
      })
    } else {
      sectors.push({
        color: slice.color,
        path: sectorPath(102.5, 102.5, outerRadius, innerRadius, startAngle, endAngle),
      })
    }
    consumed += fraction
  }

  return sectors
}
