import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { getUIVisibility, onUIVisibilityChanged } from '@/api/uiVisibility'
import { VisibilitySource } from '@/lib/uiVisibility'

export const useUIVisibilityStore = defineStore('uiVisibility', () => {
  const visible = ref(true)
  const source = new VisibilitySource(onUIVisibilityChanged, getUIVisibility, (value) => { visible.value = value })
  let started = false
  const documentChanged = (): void => {
    source.setDocumentVisible(document.visibilityState !== 'hidden')
    if (document.visibilityState !== 'hidden') void source.refresh()
  }
  const returned = (): void => { void source.refresh() }
  function start(): void {
    if (started) return
    started = true
    source.setDocumentVisible(document.visibilityState !== 'hidden')
    document.addEventListener('visibilitychange', documentChanged)
    window.addEventListener('focus', returned)
    void source.start()
  }
  function stop(): void {
    if (!started) return
    started = false
    document.removeEventListener('visibilitychange', documentChanged)
    window.removeEventListener('focus', returned)
    source.stop()
  }
  return { visible: computed(() => visible.value), start, stop }
})
