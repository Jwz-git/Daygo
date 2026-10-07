import { defineStore } from 'pinia'
import { ref } from 'vue'

import { getDiagnostics, type DiagnosticsDTO } from '@/api/diagnostics'

export const useDiagnosticsStore = defineStore('diagnostics', () => {
  const diagnostics = ref<DiagnosticsDTO | null>(null)
  const loading = ref(false)
  const loadFailed = ref(false)

  async function load(): Promise<void> {
    if (loading.value) return
    loading.value = true
    loadFailed.value = false
    try {
      diagnostics.value = await getDiagnostics()
    } catch {
      loadFailed.value = true
    } finally {
      loading.value = false
    }
  }

  return { diagnostics, loading, loadFailed, load }
})
