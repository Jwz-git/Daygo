import { ref } from 'vue'

export interface PlaygroundRequest {
  providerId: string
  model: string
  text: string
  imageType: string
  imageBase64: string
}

export interface PlaygroundResult {
  ok: boolean
  text: string
  model: string
  latencyMs: number
  errorCode: string
}

export function imageFileError(file: Pick<File, 'type' | 'size'>): '' | 'imageInvalid' {
  return ['image/png', 'image/jpeg'].includes(file.type) && file.size > 0 && file.size <= 5 * 1024 * 1024 ? '' : 'imageInvalid'
}

/** A page-local, memory-only session. Clearing never claims to cancel HTTP. */
export function createPlaygroundSession(invoke: (request: PlaygroundRequest) => Promise<PlaygroundResult>) {
  const busy = ref(false)
  const result = ref<PlaygroundResult | null>(null)
  const error = ref('')
  let generation = 0
  function clear(): void {
    generation++
    result.value = null
    error.value = ''
  }
  async function send(request: PlaygroundRequest): Promise<void> {
    if (busy.value) return
    clear()
    const current = generation
    busy.value = true
    try {
      const response = await invoke({ ...request })
      if (current === generation) result.value = response
    } catch {
      if (current === generation) error.value = 'failed'
    } finally {
      busy.value = false
    }
  }
  return { busy, result, error, send, clear }
}
