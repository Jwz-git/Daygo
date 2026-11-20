import { desktopRuntime, hasDesktopBridge } from '@/api/desktopBridge'
import { GetUIVisibility } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'

interface VisibilityRuntime {
  EventsOnMultiple?: (name: string, callback: (...data: unknown[]) => void, max: number) => (() => void) | undefined
}
export async function getUIVisibility(): Promise<boolean> {
  if (!hasDesktopBridge()) return true
  const payload: unknown = await GetUIVisibility()
  if (typeof payload !== 'object' || payload === null || !('visible' in payload) || typeof payload.visible !== 'boolean') {
    throw new Error('invalid UI visibility snapshot')
  }
  return payload.visible
}
export function onUIVisibilityChanged(callback: (visible: boolean) => void): () => void {
  const runtime = desktopRuntime()
  return runtime?.EventsOnMultiple?.('ui:visibility-changed', (...data: unknown[]) => {
    const payload = data[0]
    if (typeof payload === 'object' && payload !== null && 'visible' in payload && typeof payload.visible === 'boolean') {
      callback(payload.visible)
    }
  }, -1) ?? (() => undefined)
}
