import { GetCapabilities, TryProvider } from '../../wailsjs/go/app/Backend'
import { app } from '../../wailsjs/go/models'
import type { PlaygroundRequest, PlaygroundResult } from '@/stores/modelPlaygroundSession'

export async function tryProvider(request: PlaygroundRequest): Promise<PlaygroundResult> {
  if (!('go' in window) || window.go === undefined) throw new Error('wails_unavailable')
  return TryProvider(new app.ProviderPlaygroundRequestDTO(request))
}

export async function playgroundAccess(): Promise<boolean> {
  if (!('go' in window) || window.go === undefined) throw new Error('wails_unavailable')
  const capabilities = await GetCapabilities()
  return capabilities.canWrite && capabilities.isCaptureOwner
}

export function readImage(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('imageInvalid'))
    reader.onabort = () => reject(new Error('imageInvalid'))
    reader.onload = () => typeof reader.result === 'string' ? resolve(reader.result) : reject(new Error('imageInvalid'))
    reader.readAsDataURL(file)
  })
}
