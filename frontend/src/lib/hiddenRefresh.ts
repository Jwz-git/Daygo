import { watch } from 'vue'

/** Groups event invalidations while the window is hidden. Explicit loads
 * stay with the caller so navigation and writes retain their own semantics. */
export function deferHiddenRefresh<T>(
  isVisible: () => boolean,
  refresh: (changes: ReadonlySet<T>) => void,
  onReveal?: () => T | undefined,
) {
  let pending = new Set<T>()
  const flush = () => {
    if (!isVisible() || pending.size === 0) return
    const changes = pending
    pending = new Set<T>()
    refresh(changes)
  }
  const unwatch = watch(isVisible, (visible) => {
    if (!visible) return
    const change = onReveal?.()
    if (change !== undefined) pending.add(change)
    flush()
  }, { flush: 'sync' })
  return {
    request(change: T): void {
      pending.add(change)
      flush()
    },
    stop(): void {
      unwatch()
      pending.clear()
    },
  }
}
