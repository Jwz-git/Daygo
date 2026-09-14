import { onBeforeUnmount, onMounted, type Ref } from 'vue'

/**
 * Pointer-tracking specular highlight for lg-tracking surfaces.
 *
 * Each tracked surface registers a getBoundingClientRect() and gets its
 * own CSS custom properties --lg-mouse-x and --lg-mouse-y updated on
 * mousemove. The radial-gradient mask in liquiglass.css follows the
 * pointer and fades out when the cursor leaves the surface.
 *
 * rAF throttling caps the rate to display refresh; mousemove events
 * arrive faster than the screen can repaint, and writing CSS variables
 * outside rAF is wasted work.
 *
 * Reduced-motion: tracking is bypassed entirely; surfaces fall back to
 * the static .dg-panel specular sheen.
 */
export function usePointerHighlight(rootRef: Ref<HTMLElement | null>): {
  register: (el: HTMLElement) => void
  unregister: (el: HTMLElement) => void
} {
  let rafId = 0
  let pendingX = 0
  let pendingY = 0
  let frameDirty = false
  const tracked = new Set<HTMLElement>()

  const reducedMotion =
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches

  function writeVars(target: HTMLElement, x: number, y: number): void {
    target.style.setProperty('--lg-mouse-x', `${x}px`)
    target.style.setProperty('--lg-mouse-y', `${y}px`)
  }

  function applyFrame(): void {
    frameDirty = false
    if (tracked.size === 0) return
    for (const el of tracked) {
      const rect = el.getBoundingClientRect()
      const x = pendingX - rect.left
      const y = pendingY - rect.top
      writeVars(el, x, y)
    }
  }

  function onMove(event: PointerEvent): void {
    pendingX = event.clientX
    pendingY = event.clientY
    if (frameDirty) return
    frameDirty = true
    rafId = window.requestAnimationFrame(applyFrame)
  }

  function register(el: HTMLElement): void {
    if (reducedMotion) return
    tracked.add(el)
    el.classList.add('lg-tracking')
  }

  function unregister(el: HTMLElement): void {
    tracked.delete(el)
    el.classList.remove('lg-tracking')
  }

  onMounted(() => {
    if (reducedMotion) return
    window.addEventListener('pointermove', onMove, { passive: true })
  })

  onBeforeUnmount(() => {
    if (rafId !== 0) window.cancelAnimationFrame(rafId)
    window.removeEventListener('pointermove', onMove)
    tracked.clear()
  })

  // Return the registration helpers so callers can mark specific
  // elements. The pointer listener is global by design — registering
  // individual listeners per surface would multiply the cost of high-
  // frequency mousemove events.
  return { register, unregister }
}
