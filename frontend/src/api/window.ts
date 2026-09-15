import { SetWindowBackground } from '../../wailsjs/go/app/Backend'

/*
 * The native window backdrop is what shows through wherever the webview has
 * not painted yet — most visibly while dragging a window edge and the page
 * rasterization lags one frame behind. Reading the resolved --dg-window-bg
 * (rather than duplicating the palette here) keeps the backdrop and the page
 * on the same source of truth, so the resize gap is invisible.
 */
function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

export async function syncWindowBackground(): Promise<void> {
  if (!hasBridge()) return
  const value = getComputedStyle(document.documentElement)
    .getPropertyValue('--dg-window-bg')
    .trim()
  if (!/^#[0-9a-fA-F]{6}$/.test(value)) return
  await SetWindowBackground(value)
}
