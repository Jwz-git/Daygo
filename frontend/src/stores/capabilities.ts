import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { CapabilitiesDTO } from '@/api/dto'
import { getCapabilities } from '@/api/capabilities'

export const useCapabilitiesStore = defineStore('capabilities', () => {
  const capabilities = ref<CapabilitiesDTO | null>(null)
  const notificationsAvailable = computed(() => capabilities.value?.features.includes('notifications') ?? false)
  let inFlight: Promise<void> | null = null

  // Build/environment capabilities are stable for this process. Failed
  // discovery remains retryable and never invents an available capability.
  function load(): Promise<void> {
    if (capabilities.value !== null) return Promise.resolve()
    if (inFlight !== null) return inFlight
    inFlight = getCapabilities()
      .then((value) => { capabilities.value = value })
      .catch(() => undefined)
      .finally(() => { inFlight = null })
    return inFlight
  }

  return { capabilities, notificationsAvailable, load }
})
