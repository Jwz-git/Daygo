import { watch } from 'vue'

import { useUIVisibilityStore } from '@/stores/uiVisibility'

export function useVisibleInterval(tick: () => void, milliseconds: number, enabled = () => true): void {
  const visibility = useUIVisibilityStore()
  watch(() => visibility.visible && enabled(), (active, _previous, onCleanup) => {
    if (!active) return
    tick()
    const timer = window.setInterval(tick, milliseconds)
    onCleanup(() => window.clearInterval(timer))
  }, { immediate: true, flush: 'sync' })
}
