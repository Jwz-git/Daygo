/*
 * Daygo's interface icon set.
 *
 * One drawing system for every button and detail icon: a 24-unit grid, round
 * caps and joins, generously rounded corners, and a stroke that DgIcon keeps
 * visually even across sizes. Glyphs are path data only, drawn in
 * currentColor, so they follow text colour in both appearances. App and site
 * marks (AppSiteIcon) are brand identity, not part of this set.
 *
 * A glyph is a list of strokes; `filled` paths are solid shapes (play, pause)
 * that read better than outlines at small sizes.
 */

export interface GlyphPath {
  readonly d: string
  readonly filled?: boolean
}

export const glyphs = {
  // Navigation
  timeline: [{ d: 'M5 4v16' }, { d: 'M10.5 5h6a2.5 2.5 0 0 1 0 5h-6a2.5 2.5 0 0 1 0-5Z' }, { d: 'M10.5 14h3a2.5 2.5 0 0 1 0 5h-3a2.5 2.5 0 0 1 0-5Z' }],
  daily: [{ d: 'M7.5 5h9A3.5 3.5 0 0 1 20 8.5v8a3.5 3.5 0 0 1-3.5 3.5h-9A3.5 3.5 0 0 1 4 16.5v-8A3.5 3.5 0 0 1 7.5 5Z' }, { d: 'M4 10h16M8.5 3v3.5M15.5 3v3.5M8.5 14.5h4' }],
  weekly: [{ d: 'M4 20h16' }, { d: 'M7 16v-3.5M12 16V6.5M17 16v-6.5' }],
  chat: [{ d: 'M12 4.5c4.4 0 8 3 8 6.8s-3.6 6.8-8 6.8c-1 0-2-.2-2.9-.5L5 19.5l1.2-3.6A6.4 6.4 0 0 1 4 11.3C4 7.5 7.6 4.5 12 4.5Z' }, { d: 'M8.6 11.3h.01M12 11.3h.01M15.4 11.3h.01' }],
  settings: [{ d: 'M4 8h8.5M18.5 8H20M4 16h1.5M11.5 16H20' }, { d: 'M15.5 5.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5ZM8.5 13.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5Z' }],
  camera: [{ d: 'M4 9.5A2.5 2.5 0 0 1 6.5 7h1.2l1.5-2h5.6l1.5 2h1.2A2.5 2.5 0 0 1 20 9.5v8a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 17.5Z' }, { d: 'M12 10a3.3 3.3 0 1 1 0 6.6 3.3 3.3 0 0 1 0-6.6Z' }],

  // Direction
  chevronLeft: [{ d: 'M14.5 6 8.5 12l6 6' }],
  chevronRight: [{ d: 'M9.5 6l6 6-6 6' }],
  chevronUp: [{ d: 'M6 14.5l6-6 6 6' }],
  chevronDown: [{ d: 'M6 9.5l6 6 6-6' }],
  arrowLeft: [{ d: 'M19 12H5.5M11 6l-6 6 6 6' }],
  arrowRight: [{ d: 'M5 12h13.5M13 6l6 6-6 6' }],
  arrowUp: [{ d: 'M12 19V5.5M6 11l6-6 6 6' }],
  undo: [{ d: 'M9 14.5 4.5 10 9 5.5' }, { d: 'M4.5 10H14a5 5 0 0 1 0 10h-3' }],
  refresh: [{ d: 'M19.5 12a7.5 7.5 0 1 1-2.2-5.3L19.5 9' }, { d: 'M19.5 4.5V9H15' }],
  expand: [{ d: 'M14.5 4.5h5v5M9.5 19.5h-5v-5M19.5 4.5 14 10M4.5 19.5 10 14' }],

  // Actions
  close: [{ d: 'M6.5 6.5l11 11M17.5 6.5l-11 11' }],
  plus: [{ d: 'M12 5.5v13M5.5 12h13' }],
  minus: [{ d: 'M5.5 12h13' }],
  check: [{ d: 'M5 12.5 9.5 17 19 7.5' }],
  pencil: [{ d: 'M4.5 19.5 5.4 15 15.6 4.8a2 2 0 0 1 2.8 0l.8.8a2 2 0 0 1 0 2.8L9 18.6Z' }, { d: 'M13.5 7l3.5 3.5' }],
  trash: [{ d: 'M4.5 7h15M9.5 7V5.5A1.5 1.5 0 0 1 11 4h2a1.5 1.5 0 0 1 1.5 1.5V7' }, { d: 'M6.5 7l.8 11.2A2 2 0 0 0 9.3 20h5.4a2 2 0 0 0 2-1.8L17.5 7M10 11v5M14 11v5' }],
  copy: [{ d: 'M11 8.5h6a2.5 2.5 0 0 1 2.5 2.5v6a2.5 2.5 0 0 1-2.5 2.5h-6A2.5 2.5 0 0 1 8.5 17v-6A2.5 2.5 0 0 1 11 8.5Z' }, { d: 'M15.5 8.5V7A2.5 2.5 0 0 0 13 4.5H7A2.5 2.5 0 0 0 4.5 7v6A2.5 2.5 0 0 0 7 15.5h1.5' }],
  send: [{ d: 'M5.3 12 4.2 5.6a1 1 0 0 1 1.4-1.1l13.8 6.6a1 1 0 0 1 0 1.8L5.6 19.5a1 1 0 0 1-1.4-1.1Z' }, { d: 'M5.3 12h6.2' }],
  menu: [{ d: 'M4.5 7h15M4.5 12h15M4.5 17h15' }],
  signOut: [{ d: 'M10 4.5H7A2.5 2.5 0 0 0 4.5 7v10A2.5 2.5 0 0 0 7 19.5h3' }, { d: 'M15 8l4 4-4 4M19 12H10' }],

  // Media
  play: [{ d: 'M8 6.3v11.4a1.3 1.3 0 0 0 2 1.1l9-5.7a1.3 1.3 0 0 0 0-2.2l-9-5.7a1.3 1.3 0 0 0-2 1.1Z', filled: true }],
  pause: [{ d: 'M7.5 5.5h1.5a1.5 1.5 0 0 1 1.5 1.5v10a1.5 1.5 0 0 1-1.5 1.5H7.5A1.5 1.5 0 0 1 6 17V7a1.5 1.5 0 0 1 1.5-1.5ZM15 5.5h1.5A1.5 1.5 0 0 1 18 7v10a1.5 1.5 0 0 1-1.5 1.5H15a1.5 1.5 0 0 1-1.5-1.5V7A1.5 1.5 0 0 1 15 5.5Z', filled: true }],

  // Objects and status
  calendar: [{ d: 'M7.5 5h9A3.5 3.5 0 0 1 20 8.5v8a3.5 3.5 0 0 1-3.5 3.5h-9A3.5 3.5 0 0 1 4 16.5v-8A3.5 3.5 0 0 1 7.5 5Z' }, { d: 'M4 10h16M8.5 3v3.5M15.5 3v3.5' }, { d: 'M8.5 14.5h.01M12 14.5h.01M15.5 14.5h.01' }],
  document: [{ d: 'M7.5 3.5h5.8l4.7 4.7v10.3a2 2 0 0 1-2 2H7.5a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2Z' }, { d: 'M13 3.5V8.5h5' }],
  list: [{ d: 'M7 4.5h10A2.5 2.5 0 0 1 19.5 7v10a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 17V7A2.5 2.5 0 0 1 7 4.5Z' }, { d: 'M8.5 9h7M8.5 12h7M8.5 15h4' }],
  plusCircle: [{ d: 'M12 4a8 8 0 1 1 0 16 8 8 0 0 1 0-16Z' }, { d: 'M12 8.5v7M8.5 12h7' }],
  bars: [{ d: 'M6.5 18v-3M12 18v-7M17.5 18V7' }],
  layers: [{ d: 'M12 4 4 8l8 4 8-4-8-4Z' }, { d: 'M4 12l8 4 8-4M4 16l8 4 8-4' }],
  link: [{ d: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1' }, { d: 'M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1' }],
  sparkle: [{ d: 'M12 4c.6 4.8 3.2 7.4 8 8-4.8.6-7.4 3.2-8 8-.6-4.8-3.2-7.4-8-8 4.8-.6 7.4-3.2 8-8Z' }],
  image: [{ d: 'M7 4.5h10A2.5 2.5 0 0 1 19.5 7v10a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 17V7A2.5 2.5 0 0 1 7 4.5Z' }, { d: 'M4.5 16l4-4 3.5 3.5 2.5-2.5 5 5M15 9h.01' }],
  lock: [{ d: 'M7 10.5h10a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2v-5.5a2 2 0 0 1 2-2Z' }, { d: 'M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5' }],
  key: [{ d: 'M8.5 11a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z' }, { d: 'M11.3 12.2 19 4.5M16.5 7l2.5 2.5M14 9.5l2 2' }],
  thumbsUp: [{ d: 'M7.5 10.5v9M7.5 10.5 11 4.2a1.6 1.6 0 0 1 3 .9l-.6 4.4h4.5a2 2 0 0 1 2 2.4l-1.2 5.8a2 2 0 0 1-2 1.8H7.5M7.5 10.5H5a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h2.5' }],
  thumbsDown: [{ d: 'M7.5 13.5v-9M7.5 13.5 11 19.8a1.6 1.6 0 0 0 3-.9l-.6-4.4h4.5a2 2 0 0 0 2-2.4l-1.2-5.8a2 2 0 0 0-2-1.8H7.5M7.5 13.5H5a1 1 0 0 1-1-1v-7a1 1 0 0 1 1-1h2.5' }],
} as const satisfies Record<string, readonly GlyphPath[]>

export type IconName = keyof typeof glyphs

/*
 * Rendered stroke width in CSS pixels for a given icon size. A fixed 2-unit
 * stroke would shrink to under a pixel at 8–12px and look bold at 48px; this
 * keeps small icons legible and large ones from turning heavy.
 */
export function renderedStroke(size: number): number {
  if (size >= 28) return 2.25
  if (size >= 20) return 2
  if (size >= 15) return 1.75
  if (size >= 11) return 1.5
  return 1.25
}

/** Stroke width in viewBox units for an icon drawn at `size` CSS pixels. */
export function viewBoxStroke(size: number): number {
  return Math.round(((renderedStroke(size) * 24) / size) * 100) / 100
}
