import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { hourTicks } from '@/stores/weeklyCharts'

/** Keep SVG units in CSS pixels; only the time buckets stretch horizontally. */
export function useWeeklyGrid(
  window: () => { start: number; end: number; columns: number },
  labelWidth: number,
) {
  const container = ref<HTMLElement | null>(null)
  const width = ref(560)
  let observer: ResizeObserver | null = null

  function measure(available: number): void {
    if (Number.isFinite(available)) width.value = Math.max(560, available)
  }

  onMounted(() => {
    const element = container.value
    if (element === null) return
    measure(element.clientWidth)
    if (typeof ResizeObserver === 'undefined') return
    observer = new ResizeObserver(([entry]) => {
      if (entry) measure(entry.contentRect.width)
    })
    observer.observe(element)
  })
  onBeforeUnmount(() => observer?.disconnect())

  // Reserve room at the right edge for the last clock label.
  const plotWidth = computed(() => width.value - labelWidth - 24)
  const step = computed(() => plotWidth.value / Math.max(1, window().columns))
  const ticks = computed(() => {
    const { start, end } = window()
    const pixelsPerMinute = plotWidth.value / Math.max(1, end - start)
    const hourStride = Math.max(1, Math.ceil(48 / (60 * pixelsPerMinute)))
    return hourTicks(start, end)
      .filter((_, index) => index % hourStride === 0)
      .map((minute) => ({ minute, x: labelWidth + (minute - start) * pixelsPerMinute }))
  })

  return { container, width, step, ticks }
}
