import { GetUIVisibility } from '../../wailsjs/go/app/Backend'

interface VisibilityRuntime {
  EventsOnMultiple?: (name: string, callback: (...data: unknown[]) => void, max: number) => (() => void) | undefined
}
export async function getUIVisibility(): Promise<boolean> {
  if (!('go' in window) || window.go === undefined) return true
  const payload: unknown = await GetUIVisibility()
  if (typeof payload !== 'object' || payload === null || !('visible' in payload) || typeof payload.visible !== 'boolean') {
    throw new Error('invalid UI visibility snapshot')
  }
  return payload.visible
}
export function onUIVisibilityChanged(callback: (visible: boolean) => void): () => void {
  const runtime = (window as { runtime?: VisibilityRuntime }).runtime
  return runtime?.EventsOnMultiple?.('ui:visibility-changed', (...data: unknown[]) => {
    const payload = data[0]
    if (typeof payload === 'object' && payload !== null && 'visible' in payload && typeof payload.visible === 'boolean') {
      callback(payload.visible)
    }
  }, -1) ?? (() => undefined)
}
