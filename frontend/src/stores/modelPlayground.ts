import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import defaultImage from '@/assets/favicons/daygo.png?inline'
import { useProvidersStore } from './providers'
import { createPlaygroundSession, imageFileError } from './modelPlaygroundSession'
import { playgroundAccess, readImage, tryProvider } from '@/api/modelPlayground'

/** Page-scoped state: no storage, chat history, automatic requests or fallback. */
export function useModelPlayground() {
  const { t } = useI18n()
  const providers = useProvidersStore()
  const session = createPlaygroundSession(tryProvider)
  const providerId = ref('')
  const model = ref('')
  const text = ref(t('modelPlayground.defaultPrompt'))
  const imageURL = ref(defaultImage)
  const imageType = ref('image/png')
  const imageError = ref('')
  const reading = ref(false)
  const accessError = ref('')
  const ready = ref(false)
  let imageGeneration = 0
  let disposed = false
  const provider = computed(() => providers.providers.find((p) => p.id === providerId.value))
  const textTooLong = computed(() => Array.from(text.value).length > 16000)
  const canSend = computed(() => ready.value && !accessError.value && !session.busy.value && !reading.value &&
    !!provider.value?.hasSecret && !!provider.value?.models.includes(model.value) && !textTooLong.value &&
    !imageError.value && (!!text.value.trim() || !!imageURL.value))

  watch(provider, (value) => {
    if (!value?.models.includes(model.value)) model.value = value?.models[0] ?? ''
  })

  // A reply belongs to the exact input and selected model that produced it.
  watch([providerId, model, text, imageURL], () => {
    if (!session.busy.value) session.clear()
  })

  async function initialize(selectedProvider: unknown, selectedModel: unknown): Promise<void> {
    try {
      const writable = await playgroundAccess()
      if (disposed) return
      if (!writable) accessError.value = 'readOnly'
      await providers.hydrate()
      if (disposed) return
      if (providers.unavailable) { accessError.value = 'failed'; return }
      providerId.value = typeof selectedProvider === 'string' && providers.providers.some((p) => p.id === selectedProvider)
        ? selectedProvider : providers.providers[0]?.id ?? ''
      model.value = typeof selectedModel === 'string' && provider.value?.models.includes(selectedModel)
        ? selectedModel : provider.value?.models[0] ?? ''
      ready.value = true
    } catch {
      if (!disposed) accessError.value = 'unavailable'
    }
  }

  function removeImage(): void {
    imageGeneration++
    imageURL.value = ''
    imageType.value = ''
    imageError.value = ''
    reading.value = false
  }

  async function selectImage(file?: File): Promise<void> {
    if (!file || session.busy.value) return
    removeImage()
    imageError.value = imageFileError(file)
    if (imageError.value) return
    const current = imageGeneration
    reading.value = true
    try {
      const url = await readImage(file)
      // Decode before preview/send, including the pixel budget used by Go.
      const preview = new Image()
      preview.src = url
      await preview.decode()
      if (preview.naturalWidth * preview.naturalHeight > 20000000) throw new Error('imageInvalid')
      if (current === imageGeneration && !disposed) {
        imageURL.value = url
        imageType.value = file.type
      }
    } catch {
      if (current === imageGeneration && !disposed) imageError.value = 'imageInvalid'
    } finally {
      if (current === imageGeneration && !disposed) reading.value = false
    }
  }

  async function send(): Promise<void> {
    if (!canSend.value) return
    await session.send({ providerId: providerId.value, model: model.value, text: text.value,
      imageType: imageType.value, imageBase64: imageURL.value.split(',')[1] ?? '' })
  }

  onBeforeUnmount(() => {
    disposed = true
    removeImage()
    text.value = ''
    session.clear()
  })

  return { providers, providerId, provider, model, text, imageURL, imageError, reading, accessError,
    textTooLong, canSend, initialize, selectImage, removeImage, ...session, sendInput: send }
}
