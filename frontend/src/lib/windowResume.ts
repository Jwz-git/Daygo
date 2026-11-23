import type { WindowPlayerDTO, WindowResumeDTO } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'

type ViewSnapshot = Pick<WindowResumeDTO, 'route' | 'day' | 'cardId'>

// Lives only inside this WebView. Views whose state has no restore contract
// keep their existing window, including any stores retained after navigation.
let viewReader: (() => ViewSnapshot | null) | null = null
let player: (() => { cardId: number; state: WindowPlayerDTO }) | null = null
let pendingActions = 0
let restored: WindowResumeDTO | null = null
let cardToSelect = 0
export function takeRestoredWeek(): string {
  if (restored?.route !== 'weekly') return ''
  const day = restored.day
  restored = null
  return day
}

// Installed on Pinia before hydration. A save or generation can outlive its
// route, so its completion must be visible to the window-release handshake.
export function trackWindowAction(): () => void {
  pendingActions++
  return () => { pendingActions-- }
}

export function registerWindowView(read: NonNullable<typeof viewReader>): () => void {
  viewReader = read
  return () => { if (viewReader === read) viewReader = null }
}

export function registerWindowPlayer(read: NonNullable<typeof player>): () => void {
  player = read
  return () => { if (player === read) player = null }
}

export function windowSnapshot(): WindowResumeDTO | null {
  if (pendingActions > 0 || !viewReader) return null
  const view = viewReader()
  if (!view) return null
  const media = player?.()
  if (media && media.cardId !== view.cardId) return null
  return { ...view, player: media?.state ?? null }
}

export function restoreWindow(snapshot: WindowResumeDTO): void {
  restored = snapshot
  cardToSelect = snapshot.cardId
}
export function takeRestoredCardID(): number {
  const id = cardToSelect
  cardToSelect = 0
  return id
}
export function takeRestoredPlayer(cardId: number): WindowPlayerDTO | null {
  if (!restored || restored.cardId !== cardId) return null
  const state = restored.player
  restored = null
  return state
}
