import { onBeforeUnmount, onMounted } from 'vue'

/*
 * SVG displacement filter singleton.
 *
 * Per fu-liquiglass skill, refraction must be progressive enhancement:
 *   - never apply to text (only to compact surfaces or decorative layers)
 *   - degrade cleanly when the browser cannot composite the filter
 *   - respect prefers-reduced-motion
 *   - respect the global data-dg-lg-refraction attribute that callers
 *     can set to 'disabled' to turn the effect off without a rebuild
 *
 * The filter itself is a static feTurbulence + feDisplacementMap. It
 * lives in a hidden SVG; surfaces that want refraction reference the
 * filter id via CSS `filter: url(#lg-displacement)`.
 */

export type RefractionIntensity = 'subtle' | 'normal' | 'off'

const SVG_NS = 'http://www.w3.org/2000/svg'

let installed = false
let currentIntensity: RefractionIntensity = 'subtle'

function buildSvg(intensity: RefractionIntensity): SVGSVGElement {
  const svg = document.createElementNS(SVG_NS, 'svg')
  svg.setAttribute('aria-hidden', 'true')
  svg.setAttribute('width', '0')
  svg.setAttribute('height', '0')
  svg.style.position = 'absolute'
  svg.style.width = '0'
  svg.style.height = '0'

  // Scale grows with intensity but is bounded — anything above ~14 starts
  // to look like static, not glass.
  const scale = intensity === 'subtle' ? 6 : intensity === 'normal' ? 10 : 0

  const filter = document.createElementNS(SVG_NS, 'filter')
  filter.id = 'lg-displacement'
  filter.setAttribute('x', '-10%')
  filter.setAttribute('y', '-10%')
  filter.setAttribute('width', '120%')
  filter.setAttribute('height', '120%')

  const turbulence = document.createElementNS(SVG_NS, 'feTurbulence')
  turbulence.setAttribute('type', 'fractalNoise')
  turbulence.setAttribute('baseFrequency', '0.012 0.018')
  turbulence.setAttribute('numOctaves', '2')
  turbulence.setAttribute('seed', '7')
  turbulence.setAttribute('result', 'noise')

  const displacement = document.createElementNS(SVG_NS, 'feDisplacementMap')
  displacement.setAttribute('in', 'SourceGraphic')
  displacement.setAttribute('in2', 'noise')
  displacement.setAttribute('scale', String(scale))
  displacement.setAttribute('xChannelSelector', 'R')
  displacement.setAttribute('yChannelSelector', 'B')

  filter.append(turbulence, displacement)
  svg.append(filter)
  return svg
}

export function installRefractionFilter(intensity: RefractionIntensity = 'subtle'): void {
  if (installed) return
  if (typeof document === 'undefined') return

  const reducedMotion =
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches

  if (reducedMotion) return
  if (document.documentElement.dataset['dgLgRefraction'] === 'disabled') return

  const svg = buildSvg(intensity)
  document.body.append(svg)
  installed = true
  currentIntensity = intensity
}

export function setRefractionIntensity(intensity: RefractionIntensity): void {
  if (!installed) {
    installRefractionFilter(intensity)
    return
  }
  if (intensity === 'off') {
    const existing = document.getElementById('lg-displacement-host')
    if (existing) existing.remove()
    installed = false
    return
  }
  if (intensity === currentIntensity) return
  const existing = document.getElementById('lg-displacement-host')
  if (existing) existing.remove()
  buildSvg(intensity)
  installed = true
  currentIntensity = intensity
}

/**
 * Composable for AppShell — installs the filter once on mount, removes
 * it on unmount. Callers should call setRefractionIntensity to swap
 * intensities without remounting.
 */
export function useRefractionFilter(intensity: RefractionIntensity = 'subtle'): void {
  onMounted(() => installRefractionFilter(intensity))
  onBeforeUnmount(() => setRefractionIntensity('off'))
}
