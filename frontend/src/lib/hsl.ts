/*
 * HSL → hex, a direct port of the reference implementation's helper
 * (ColorWheelPicker.swift hslToRGB/hslToHex). Hue is normalized modulo 360,
 * so callers can hand it angles outside [0, 360) — the color canvas derives
 * hues from a drag angle without wrapping them first.
 */

function hslToRgb(h: number, s: number, l: number): [number, number, number] {
  const hue = ((h % 360) + 360) % 360
  const saturation = Math.max(0, Math.min(100, s)) / 100
  const lightness = Math.max(0, Math.min(100, l)) / 100

  const k = (n: number): number => (n + hue / 30) % 12
  const a = saturation * Math.min(lightness, 1 - lightness)
  const f = (n: number): number =>
    lightness - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)))

  return [f(0), f(8), f(4)]
}

/** Uppercase "#RRGGBB", matching the reference hex strings. */
export function hslToHex(h: number, s: number, l: number): string {
  const [r, g, b] = hslToRgb(h, s, l)
  const channel = (value: number): string =>
    Math.max(0, Math.min(255, Math.round(value * 255))).toString(16).padStart(2, '0')
  return `#${channel(r)}${channel(g)}${channel(b)}`.toUpperCase()
}
