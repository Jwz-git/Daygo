import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { getTokenUsage, type TokenUsage } from '@/api/tokenUsage'

// A separate read failure must not hide the daily/weekly report. Versioning
// prevents a slow response from a previous date replacing the current one.
export function useTokenUsage(period: 'day' | 'week', day: Ref<string>) {
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
  watch(day, () => { usage.value = null; void reload() }, { immediate: true })
  // Refresh while visible, including calls that don't emit timeline events.
  const timer = window.setInterval(() => { void reload() }, 60_000)
  onBeforeUnmount(() => { version++; window.clearInterval(timer) })
  return { usage, state, reload }
}
