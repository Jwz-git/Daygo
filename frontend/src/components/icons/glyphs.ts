/*
 * Daygo's interface icons, matched to Dayflow's.
 *
 * Dayflow draws icons two ways, and so does this set:
 *
 *   - Its own artwork (sidebar, copy, edit, calendar, thumbs up, delete) ships
 *     as image assets rendered as templates: the shape comes from the file,
 *     the colour from the theme. Those files are reused here unchanged (Dayflow
 *     is MIT licensed, Copyright (c) 2025 Jerry Liu) under assets/icons/dayflow
 *     and drawn through an alpha mask filled with currentColor, so they tint
 *     exactly like Dayflow's templates.
 *   - Everything else is an SF Symbol in Dayflow. SF Symbols may only be used on
 *     Apple platforms and Daygo also ships on Windows, so those are redrawn
 *     here on a 24-unit grid to the same shapes and weights: line glyphs for
 *     the plain symbols, solid shapes where Dayflow uses the `.fill` variant.
 *
 * App and site brand marks (AppSiteIcon) are not part of this set.
 */

import calendarAsset from '@/assets/icons/dayflow/calendar.png'
import chatAsset from '@/assets/icons/dayflow/chat.svg'
import copyAsset from '@/assets/icons/dayflow/copy.svg'
import dailyAsset from '@/assets/icons/dayflow/daily.svg'
import deleteAsset from '@/assets/icons/dayflow/delete.png'
import editAsset from '@/assets/icons/dayflow/edit.svg'
import thumbsUpAsset from '@/assets/icons/dayflow/thumbs-up.png'
import timelineAsset from '@/assets/icons/dayflow/timeline.png'
import weeklyAsset from '@/assets/icons/dayflow/weekly.svg'

export interface GlyphPath {
  readonly d: string
  /** A solid shape (with even-odd holes) rather than a stroked line. */
  readonly filled?: boolean
}

export type Glyph =
  | { readonly kind: 'vector'; readonly paths: readonly GlyphPath[] }
  | { readonly kind: 'asset'; readonly src: string; readonly flipY?: boolean }

const vector = (...paths: GlyphPath[]): Glyph => ({ kind: 'vector', paths })
const asset = (src: string, flipY = false): Glyph => ({ kind: 'asset', src, flipY })

export const glyphs = {
  // Navigation — Dayflow's own sidebar artwork; settings is gearshape.fill.
  timeline: asset(timelineAsset),
  daily: asset(dailyAsset),
  weekly: asset(weeklyAsset),
  chat: asset(chatAsset),
  settings: vector({
    d: 'M12 4.6 13.04 4.67 13.95 2.6 15.6 3.1 15.2 5.33 16.11 5.85 17.23 6.77 17.92 7.56 20.02 6.73 20.84 8.26 18.98 9.55 19.26 10.56 19.4 12 19.33 13.04 21.4 13.95 20.9 15.6 18.67 15.2 18.15 16.11 17.23 17.23 16.44 17.92 17.27 20.02 15.74 20.84 14.45 18.98 13.44 19.26 12 19.4 10.96 19.33 10.05 21.4 8.4 20.9 8.8 18.67 7.89 18.15 6.77 17.23 6.08 16.44 3.98 17.27 3.16 15.74 5.02 14.45 4.74 13.44 4.6 12 4.67 10.96 2.6 10.05 3.1 8.4 5.33 8.8 5.85 7.89 6.77 6.77 7.56 6.08 6.73 3.98 8.26 3.16 9.55 5.02 10.56 4.74ZM15.2 12a3.2 3.2 0 1 0-6.4 0 3.2 3.2 0 1 0 6.4 0Z',
    filled: true,
  }),
  camera: vector({
    d: 'M3.5 9A2.5 2.5 0 0 1 6 6.5h1.6l1.3-1.8A1.6 1.6 0 0 1 10.2 4h3.6a1.6 1.6 0 0 1 1.3.7l1.3 1.8H18A2.5 2.5 0 0 1 20.5 9v8.5A2.5 2.5 0 0 1 18 20H6a2.5 2.5 0 0 1-2.5-2.5ZM12 9.3a3.9 3.9 0 1 0 0 7.8 3.9 3.9 0 0 0 0-7.8Z',
    filled: true,
  }),

  // Direction — chevron.*, arrow.*, arrow.uturn.backward, arrow.clockwise.
  chevronLeft: vector({ d: 'M15 5 8 12l7 7' }),
  chevronRight: vector({ d: 'M9 5l7 7-7 7' }),
  chevronUp: vector({ d: 'M5 15l7-7 7 7' }),
  chevronDown: vector({ d: 'M5 9l7 7 7-7' }),
  arrowLeft: vector({ d: 'M20 12H4.5M10.5 5.5 4 12l6.5 6.5' }),
  arrowRight: vector({ d: 'M4 12h15.5M13.5 5.5 20 12l-6.5 6.5' }),
  arrowUp: vector({ d: 'M12 20V4.5M5.5 10.5 12 4l6.5 6.5' }),
  undo: vector({ d: 'M8.5 14.5 3.5 9.5l5-5' }, { d: 'M3.5 9.5H15a5.5 5.5 0 0 1 0 11h-3' }),
  refresh: vector({ d: 'M20 12a8 8 0 1 1-2.34-5.66L20 8.5' }, { d: 'M20 3.5v5h-5' }),
  expand: vector({ d: 'M14 4h6v6M10 20H4v-6M20 4l-6.5 6.5M4 20l6.5-6.5' }),

  // Actions — xmark, plus, minus, checkmark, square.and.pencil, paperplane.fill.
  close: vector({ d: 'M6 6l12 12M18 6 6 18' }),
  plus: vector({ d: 'M12 4.5v15M4.5 12h15' }),
  minus: vector({ d: 'M4.5 12h15' }),
  check: vector({ d: 'M4.5 12.5 9.5 17.5 19.5 6.5' }),
  pencil: asset(editAsset),
  trash: asset(deleteAsset),
  copy: asset(copyAsset),
  send: vector({
    d: 'M3.4 4.3a1 1 0 0 1 1.3-1.2l15.9 7.9a1.1 1.1 0 0 1 0 2L4.7 20.9a1 1 0 0 1-1.3-1.2L5.5 12Zm2.9 6.8h6.5a.9.9 0 0 1 0 1.8H6.3Z',
    filled: true,
  }),
  menu: vector({ d: 'M4 6.5h16M4 12h16M4 17.5h16' }),
  signOut: vector({ d: 'M10 4H7a3 3 0 0 0-3 3v10a3 3 0 0 0 3 3h3' }, { d: 'M15 7.5 19.5 12 15 16.5M19.5 12H10' }),

  // Media — play.fill, pause.fill.
  play: vector({ d: 'M7.5 5.7v12.6a1.4 1.4 0 0 0 2.1 1.2l10-6.3a1.4 1.4 0 0 0 0-2.4l-10-6.3a1.4 1.4 0 0 0-2.1 1.2Z', filled: true }),
  pause: vector({
    d: 'M7 5h2a1.5 1.5 0 0 1 1.5 1.5v11A1.5 1.5 0 0 1 9 19H7a1.5 1.5 0 0 1-1.5-1.5v-11A1.5 1.5 0 0 1 7 5ZM15 5h2a1.5 1.5 0 0 1 1.5 1.5v11A1.5 1.5 0 0 1 17 19h-2a1.5 1.5 0 0 1-1.5-1.5v-11A1.5 1.5 0 0 1 15 5Z',
    filled: true,
  }),

  // Objects and status — calendar and thumbs are Dayflow artwork; the rest
  // follow doc.fill, list.bullet.rectangle, plus.circle.fill, chart.bar.fill,
  // square.stack.3d.up, link, sparkles, photo, lock.fill and key.fill.
  calendar: asset(calendarAsset),
  thumbsUp: asset(thumbsUpAsset),
  thumbsDown: asset(thumbsUpAsset, true),
  document: vector({ d: 'M7 2.5h6.2a2 2 0 0 1 1.4.6l4.3 4.3a2 2 0 0 1 .6 1.4V19a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 19V5A2.5 2.5 0 0 1 7 2.5Zm6 1.6V7.5A1.5 1.5 0 0 0 14.5 9h3.4Z', filled: true }),
  list: vector({ d: 'M6.5 4h11A2.5 2.5 0 0 1 20 6.5v11a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 17.5v-11A2.5 2.5 0 0 1 6.5 4Z' }, { d: 'M8 9h8M8 12h8M8 15h5' }),
  plusCircle: vector({ d: 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18Zm0 4.5a.9.9 0 0 0-.9.9v2.7H8.4a.9.9 0 0 0 0 1.8h2.7v2.7a.9.9 0 0 0 1.8 0v-2.7h2.7a.9.9 0 0 0 0-1.8h-2.7V8.4a.9.9 0 0 0-.9-.9Z', filled: true }),
  bars: vector({ d: 'M5 14.5a1.5 1.5 0 0 1 3 0V19a1.5 1.5 0 0 1-3 0ZM10.5 9a1.5 1.5 0 0 1 3 0v10a1.5 1.5 0 0 1-3 0ZM16 5.5a1.5 1.5 0 0 1 3 0V19a1.5 1.5 0 0 1-3 0Z', filled: true }),
  layers: vector({ d: 'M12 3.5 3.5 8 12 12.5 20.5 8Z' }, { d: 'M3.5 12 12 16.5 20.5 12M3.5 16 12 20.5 20.5 16' }),
  link: vector({ d: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1' }, { d: 'M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1' }),
  sparkle: vector({ d: 'M10 3c.7 4.4 2.6 6.3 7 7-4.4.7-6.3 2.6-7 7-.7-4.4-2.6-6.3-7-7 4.4-.7 6.3-2.6 7-7ZM18 14.5c.3 1.9 1.1 2.7 3 3-1.9.3-2.7 1.1-3 3-.3-1.9-1.1-2.7-3-3 1.9-.3 2.7-1.1 3-3Z', filled: true }),
  image: vector({ d: 'M6.5 4h11A2.5 2.5 0 0 1 20 6.5v11a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 17.5v-11A2.5 2.5 0 0 1 6.5 4Z' }, { d: 'M4 16l4.5-4.5 4 4 2.5-2.5L20 18M15.5 8.5h.01' }),
  lock: vector({ d: 'M7 10h10a2.5 2.5 0 0 1 2.5 2.5V18a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 18v-5.5A2.5 2.5 0 0 1 7 10Z', filled: true }, { d: 'M8 10V7.5a4 4 0 0 1 8 0V10' }),
  key: vector({ d: 'M8.5 10a4.5 4.5 0 1 1 0 9 4.5 4.5 0 0 1 0-9Zm0 3a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3Z', filled: true }, { d: 'M11.6 11.4 19.5 3.5M16.5 6.5l2.5 2.5M14 9l2 2' }),
} as const satisfies Record<string, Glyph>

export type IconName = keyof typeof glyphs

/*
 * Rendered stroke width in CSS pixels for a given icon size, so small line
 * icons stay legible and large ones do not turn heavy (SF Symbols' regular
 * weight sits around 1.6–2px at interface sizes).
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
