import { hasDesktopBridge } from '@/api/desktopBridge'
import { GetCapabilities, TryProvider } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'
import type * as app from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'
import type { PlaygroundRequest, PlaygroundResult } from '@/stores/modelPlaygroundSession'

export async function tryProvider(request: PlaygroundRequest): Promise<PlaygroundResult> {
  if (!hasDesktopBridge()) throw new Error('wails_unavailable')
  return TryProvider(request)
}

export async function playgroundAccess(): Promise<boolean> {
  if (!hasDesktopBridge()) throw new Error('wails_unavailable')
  const capabilities = await GetCapabilities()
  return capabilities.canWrite && capabilities.isCaptureOwner
}

export function readImage(file: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('imageInvalid'))
    reader.onabort = () => reject(new Error('imageInvalid'))
    reader.onload = () => typeof reader.result === 'string' ? resolve(reader.result) : reject(new Error('imageInvalid'))
    reader.readAsDataURL(file)
  })
}
