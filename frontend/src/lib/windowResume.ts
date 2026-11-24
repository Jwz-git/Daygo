import { shallowRef } from 'vue'
import type { WindowPlayerDTO, WindowResumeDTO } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'

type ViewSnapshot = Pick<WindowResumeDTO, 'route' | 'day' | 'cardId'>

// Lives only inside this WebView. Views whose state has no restore contract
// keep their existing window, including any stores retained after navigation.
const viewReader = shallowRef<() => ViewSnapshot | null>()
const player = shallowRef<() => { cardId: number; state: WindowPlayerDTO }>()
let pendingActions = 0
const actionsIdle = shallowRef(true)
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
  actionsIdle.value = false
  pendingActions++
  return () => { actionsIdle.value = --pendingActions === 0 }
}

export function registerWindowView(read: NonNullable<typeof viewReader.value>): () => void {
  viewReader.value = read
  return () => { if (viewReader.value === read) viewReader.value = undefined }
}

export function registerWindowPlayer(read: NonNullable<typeof player.value>): () => void {
  player.value = read
  return () => { if (player.value === read) player.value = undefined }
}

export function windowSnapshot(): WindowResumeDTO | null {
  if (!actionsIdle.value || !viewReader.value) return null
  const view = viewReader.value()
  if (!view) return null
  const media = player.value?.()
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
