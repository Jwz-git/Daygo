import { ConfirmWindowRelease, TakeWindowResumeState } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'
import { desktopRuntime, hasDesktopBridge } from './desktopBridge'
import type { WindowResumeDTO } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'
import { restoreWindow, windowSnapshot } from '@/lib/windowResume'

export async function recoverWindow(): Promise<WindowResumeDTO | null> {
  if (!hasDesktopBridge() || new URLSearchParams(location.search).get('resume') !== '1') return null
  const snapshot = await TakeWindowResumeState().catch(() => null)
  history.replaceState(history.state, '', location.pathname + location.hash)
  if (snapshot) restoreWindow(snapshot)
  return snapshot
}

export function onWindowRelease(canRelease: () => boolean): () => void {
  return desktopRuntime()?.EventsOnMultiple('ui:prepare-window-release', (token) => {
    if (typeof token !== 'number' || !Number.isSafeInteger(token) || token <= 0 || !canRelease()) return
    const snapshot = windowSnapshot()
    if (snapshot) void ConfirmWindowRelease(token, snapshot).catch(() => {})
  }, -1) ?? (() => {})
}
