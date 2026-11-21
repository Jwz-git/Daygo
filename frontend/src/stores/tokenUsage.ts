import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { useVisibleInterval } from '@/lib/visibleInterval'
import { useUIVisibilityStore } from '@/stores/uiVisibility'
import { getTokenUsage, type TokenUsage } from '@/api/tokenUsage'

// A separate read failure must not hide the daily/weekly report. Versioning
// prevents a slow response from a previous date replacing the current one.
export function useTokenUsage(period: 'day' | 'week', day: Ref<string>) {
  const visibility = useUIVisibilityStore()
  const usage = ref<TokenUsage | null>(null)
  const state = ref<'loading' | 'ready' | 'failed'>('loading')
  let version = 0
  async function reload() {
    const current = ++version
    if (!usage.value) state.value = 'loading'
    if (!day.value) return
    try {
      const result = await getTokenUsage(period, day.value)
      if (current !== version) return
      usage.value = result
      state.value = 'ready'
    } catch {
      if (current === version) state.value = 'failed'
    }
  }
  watch(day, () => {
    usage.value = null
    version++
    if (visibility.visible) void reload()
  })
  useVisibleInterval(() => { void reload() }, 60_000)
  onBeforeUnmount(() => { version++ })
  return { usage, state, reload }
}
