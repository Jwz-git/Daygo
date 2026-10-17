import assert from 'node:assert/strict'
import test from 'node:test'

import {
  CATEGORY_PALETTES,
  CATEGORY_PALETTE_IDS,
  SWATCH_COUNT,
  categoryPalette,
  paletteLightness,
  paletteSpread,
  spectrumPalette,
} from '../src/lib/colorPalette'
import { hslToHex } from '../src/lib/hsl'

/*
 * The values below were computed independently (Python colorsys.hls_to_rgb
 * with round-half-away-from-zero) rather than read back from this code, so a
 * mistyped formula in the port fails here instead of shipping a palette that
 * merely looks plausible.
 */

test('hslToHex matches the independently computed HSL reference', () => {
  assert.equal(hslToHex(270, 100, 63), '#A142FF')
  assert.equal(hslToHex(0, 100, 50), '#FF0000')
  assert.equal(hslToHex(120, 100, 50), '#00FF00')
  assert.equal(hslToHex(30, 100, 15), '#4D2600')
  assert.equal(hslToHex(200, 100, 90), '#CCEEFF')
  assert.equal(hslToHex(240, 100, 33), '#0000A8')
})

test('a negative or past-360 hue wraps instead of going out of gamut', () => {
  // The canvas keeps its angle at -π/2 until the first drag, i.e. hue -90.
  assert.equal(hslToHex(-90, 100, 63), hslToHex(270, 100, 63))
  assert.equal(hslToHex(405, 100, 50), '#FFBF00')
})

test('palette lightness ramps from 0 at the center to 90 at the rim', () => {
  assert.equal(paletteLightness(0), 0)
  assert.equal(paletteLightness(1), 90)
  assert.equal(Math.round(paletteLightness(0.7)), 63)
})

test('palette spread narrows as the drag moves outward', () => {
  assert.ok(Math.abs(paletteSpread(0.7) - 0.694082536933105) < 1e-12)
  assert.ok(Math.abs(paletteSpread(0) - 0.8377580409572781) < 1e-12)
  assert.ok(Math.abs(paletteSpread(1) - 0.41887902047863906) < 1e-12)
  // Wider at the center, narrower at the rim: the bullets fan out.
  assert.ok(paletteSpread(0) > paletteSpread(0.7))
  assert.ok(paletteSpread(0.7) > paletteSpread(1))
})

test('the spectrum palette is eight hues at 45° steps from the drag angle', () => {
  assert.equal(SWATCH_COUNT, 8)
  const palette = spectrumPalette(-Math.PI / 2, 0.7)
  assert.deepEqual(palette, [
    '#A142FF', '#FF42D0', '#FF4242', '#FFD042',
    '#A1FF42', '#42FF71', '#42FFFF', '#4271FF',
  ])
  // Same angle, dimmer radius: the hues hold, the lightness drops.
  assert.deepEqual(spectrumPalette(0, 0.2), [
    '#5C0000', '#5C4500', '#2E5C00', '#005C17',
    '#005C5C', '#00175C', '#2E005C', '#5C0045',
  ])
})

test('category theme palettes are five sets of six distinct #RRGGBB colors', () => {
  assert.deepEqual(CATEGORY_PALETTES.map((palette) => palette.id), [...CATEGORY_PALETTE_IDS])
  for (const palette of CATEGORY_PALETTES) {
    assert.equal(palette.colors.length, 6, `${palette.id} has ${palette.colors.length} colors`)
    for (const color of palette.colors) assert.match(color, /^#[0-9A-F]{6}$/, `${palette.id}: ${color}`)
    const unique = new Set(palette.colors.map((color) => color.toUpperCase()))
    assert.equal(unique.size, palette.colors.length, `${palette.id} repeats a color`)
    assert.equal(categoryPalette(palette.id), palette)
  }
})

test('every palette color reads on both the light and the dark timeline', () => {
  // Relative luminance (WCAG): the mid-tone band keeps a category legible as
  // a bar on the near-white light track and the near-black dark one alike.
  function luminance(hex: string): number {
    const channel = (offset: number): number => {
      const value = Number.parseInt(hex.slice(offset, offset + 2), 16) / 255
      return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
    }
    return 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5)
  }
  for (const palette of CATEGORY_PALETTES) {
    for (const color of palette.colors) {
      const value = luminance(color)
      assert.ok(value >= 0.07 && value <= 0.65, `${palette.id} ${color} luminance ${value.toFixed(3)}`)
    }
  }
})
