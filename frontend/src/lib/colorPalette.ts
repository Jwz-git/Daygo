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

/*
 * Category theme palettes: six mid-tone colors each, ordered so neighbours
 * contrast and none washes out on the light or the dark timeline. Applying a
 * palette recolors the user categories in display order, cycling (the
 * ApplyCategoryPalette binding); a profession preset takes its palette's
 * colors by position. Names are i18n keys under presets.palette.names.
 */
export const CATEGORY_PALETTE_IDS = [
  'daygo-classic',
  'forest-moss',
  'cyber-neon',
  'nordic-mist',
  'sunset-clay',
] as const
export type CategoryPaletteId = (typeof CATEGORY_PALETTE_IDS)[number]

export interface CategoryPalette {
  id: CategoryPaletteId
  /** Key under presets.palette.names. */
  nameKey: string
  colors: readonly string[]
}

export const CATEGORY_PALETTES: readonly CategoryPalette[] = [
  {
    id: 'daygo-classic',
    nameKey: 'daygoClassic',
    colors: ['#6A7EFF', '#FF9A6C', '#4CC4E6', '#B884F2', '#F2B33D', '#5DBE96'],
  },
  {
    id: 'forest-moss',
    nameKey: 'forestMoss',
    colors: ['#4F8A62', '#86B89A', '#C9A15B', '#6F9C9A', '#B5707E', '#98A65E'],
  },
  {
    id: 'cyber-neon',
    nameKey: 'cyberNeon',
    colors: ['#6366F1', '#06B6D4', '#EC4899', '#10B981', '#F59E0B', '#8B5CF6'],
  },
  {
    id: 'nordic-mist',
    nameKey: 'nordicMist',
    colors: ['#5E81AC', '#88C0D0', '#A3BE8C', '#EBCB8B', '#D08770', '#B48EAD'],
  },
  {
    id: 'sunset-clay',
    nameKey: 'sunsetClay',
    colors: ['#D0624C', '#E59866', '#E3BE6A', '#6C9A8B', '#546A7B', '#9E6B8E'],
  },
]

export function categoryPalette(id: CategoryPaletteId): CategoryPalette {
  const palette = CATEGORY_PALETTES.find((candidate) => candidate.id === id)
  if (palette === undefined) throw new Error(`unknown category palette: ${id}`)
  return palette
}
