import { hslToHex } from './hsl'

/*
 * Palette math for the category color stage, ported from the reference
 * implementation (ColorPickerView + ColorOrganizerRoot.spectrumColors):
 *
 *   lightness = maxLight * normalizedRadius            (0 at the center)
 *   spread    = (minSpread + (maxSpread - minSpread) * normalizedRadius^3) * spreadFactor
 *   swatches  = eight hues at 45° steps from the drag angle
 *
 * Kept here rather than inside the components so the numbers can be pinned by
 * tests; the canvas and the modal both read from this module.
 */

export const SWATCH_COUNT = 8
export const MIN_LIGHT = 15
export const MAX_LIGHT = 90
const SPREAD_FACTOR = 0.4
const MIN_SPREAD = Math.PI / 1.5
const MAX_SPREAD = Math.PI / 3

/** Lightness of the palette at a normalized drag radius (0…1). */
export function paletteLightness(normalizedRadius: number): number {
  return MAX_LIGHT * normalizedRadius
}

/** Angular half-width between the palette bullets at a normalized radius. */
export function paletteSpread(normalizedRadius: number): number {
  const eased = Math.pow(normalizedRadius, 3)
  return (MIN_SPREAD + (MAX_SPREAD - MIN_SPREAD) * eased) * SPREAD_FACTOR
}

/** The eight swatches the color stage offers for a canvas position. */
export function spectrumPalette(angle: number, normalizedRadius: number): string[] {
  const lightness = paletteLightness(normalizedRadius)
  return Array.from({ length: SWATCH_COUNT }, (_, index) => {
    const hue = ((angle + (index * 2 * Math.PI) / SWATCH_COUNT) * 180) / Math.PI
    return hslToHex(hue, 100, lightness)
  })
}
