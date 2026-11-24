import { shallowRef, triggerRef, watchPostEffect } from 'vue'
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

export function onWindowRelease(isVisible: () => boolean, canRelease: () => boolean): () => void {
  let cancelPending = () => {}
  const unsubscribe = desktopRuntime()?.EventsOnMultiple('ui:prepare-window-release', (token) => {
    if (typeof token !== 'number' || !Number.isSafeInteger(token) || token <= 0) return
    cancelPending()
    // Guards include both Vue state and rendered editor/dialog attributes.
    // Observe them only while this red-close request is pending.
    const domChanged = shallowRef(0)
    const observer = new MutationObserver(() => triggerRef(domChanged))
    observer.observe(document.body, {
      subtree: true, childList: true, attributes: true,
      attributeFilter: ['role', 'aria-busy', 'contenteditable', 'class', 'data-journal-field', 'data-retain-window'],
    })
    const stopCheck = watchPostEffect(() => {
      void domChanged.value
      if (isVisible()) {
        cancelPending()
        return
      }
      if (!canRelease()) return
      const snapshot = windowSnapshot()
      if (snapshot) {
        cancelPending()
        void ConfirmWindowRelease(token, snapshot).catch(() => {})
      }
    })
    cancelPending = () => {
      observer.disconnect()
      stopCheck()
      cancelPending = () => {}
    }
  }, -1)
  return () => { unsubscribe?.(); cancelPending() }
}
