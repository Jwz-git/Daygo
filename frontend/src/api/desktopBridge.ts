import { Browser, Events, System, Window as NativeWindow } from '@wailsio/runtime'
import * as Backend from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'

const nativeEnvironments = new WeakMap<object, unknown>()

/** Navigation can finish after npm runtime modules have executed. Query the
 * native transport before mounting so early settings reads never use preview
 * fallbacks while Wails is still injecting its environment configuration. */
export async function initializeDesktopBridge(): Promise<void> {
  if (typeof window === 'undefined' || hasDesktopBridge()) return
  const host = window as unknown as {
    webkit?: { messageHandlers?: { external?: { postMessage?: unknown } } }
    chrome?: { webview?: { postMessage?: unknown } }
  }
  if (typeof host.webkit?.messageHandlers?.external?.postMessage !== 'function' &&
      typeof host.chrome?.webview?.postMessage !== 'function') return
  const environment: unknown = await System.Environment()
  if (!isDesktopEnvironment(environment)) throw new Error('invalid desktop environment')
  nativeEnvironments.set(window, environment)
}

/** The runtime module itself also runs in Vite previews. Only the native
 * environment injected by the host establishes that Go calls are available. */
export function hasDesktopBridge(): boolean {
  if (typeof window === 'undefined') return false
  return isDesktopEnvironment(nativeEnvironments.get(window)) ||
    isDesktopEnvironment((window as unknown as { _wails?: { environment?: unknown } })._wails?.environment)
}

export function isDesktopEnvironment(value: unknown): boolean {
  if (typeof value !== 'object' || value === null || !('OS' in value)) return false
  return value.OS === 'darwin' || value.OS === 'windows' || value.OS === 'linux'
}

export function desktopBackend(): typeof Backend | null {
  return hasDesktopBridge() ? Backend : null
}

/** Adapt v3's event envelope once; application callbacks keep their DTO shape. */
export function desktopRuntime() {
  if (!hasDesktopBridge()) return undefined
  return {
    EventsOnMultiple: (name: string, callback: (...payload: unknown[]) => void, max: number): (() => void) =>
      Events.OnMultiple(name, (event) => callback(event.data), max),
  }
}

export async function desktopEnvironment(): Promise<unknown> {
  const result = await System.Environment()
  return { platform: result.OS }
}

export function openDesktopURL(url: string): void { void Browser.OpenURL(url) }
export function hideDesktopWindow(): void { void NativeWindow.Hide() }
export function minimiseDesktopWindow(): void { void NativeWindow.Minimise() }
export function toggleDesktopMaximise(): void { void NativeWindow.ToggleMaximise() }
export function desktopIsMaximised(): Promise<boolean> { return NativeWindow.IsMaximised() }
