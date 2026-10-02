import { ref } from 'vue'

/*
 * Pointer state shared by the weekly charts: which item is under the pointer
 * and where the tooltip should sit inside the chart's container. Coordinates
 * are relative to `container`; x is clamped so the tooltip never leaves the
 * card horizontally.
 */
export function useChartPointer<T>() {
  const container = ref<HTMLElement | null>(null)
  const hovered = ref<T | null>(null) as { value: T | null }
  const x = ref(0)
  const y = ref(0)

  function move(event: PointerEvent | MouseEvent, item: T): void {
    const box = container.value?.getBoundingClientRect()
    if (box === undefined) return
    const margin = Math.min(110, box.width / 2)
    x.value = Math.min(Math.max(event.clientX - box.left, margin), box.width - margin)
    y.value = event.clientY - box.top
    hovered.value = item
  }

  function leave(): void {
    hovered.value = null
  }

  return { container, hovered, x, y, move, leave }
}

/** The pointer position in the svg's own viewBox units, or null when it cannot be mapped. */
export function svgPoint(event: PointerEvent | MouseEvent): { x: number; y: number } | null {
  const svg = event.currentTarget
  if (!(svg instanceof SVGSVGElement)) return null
  const matrix = svg.getScreenCTM()
  if (matrix === null) return null
  const point = new DOMPoint(event.clientX, event.clientY).matrixTransform(matrix.inverse())
  return { x: point.x, y: point.y }
}
