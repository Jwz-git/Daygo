import type { AppTheme } from '@/api/dto'

/*
 * Appearance mechanics — the theme counterpart to i18n/index.ts setLocale().
 *
 * The stored preference is three-state (system|light|dark). What reaches the
 * DOM is always RESOLVED to one of
 * two values, written as <html data-dg-appearance="light|dark">. Resolving
 * before the write is what lets styles/tokens.dark.css be a single attribute
 * block with no prefers-color-scheme duplicate, and it is what makes an
 * explicit Light choice stick on a dark system.
 *
 * Go through useAppearanceStore().setTheme() rather than calling these directly.
 */

export type ResolvedAppearance = 'light' | 'dark'

export const DEFAULT_THEME: AppTheme = 'system'

const DARK_QUERY = '(prefers-color-scheme: dark)'

function darkMediaQuery(): MediaQueryList | null {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return null
  }
  return window.matchMedia(DARK_QUERY)
}

export function systemAppearance(): ResolvedAppearance {
  return darkMediaQuery()?.matches ? 'dark' : 'light'
}

export function resolveAppearance(theme: AppTheme): ResolvedAppearance {
  return theme === 'system' ? systemAppearance() : theme
}

export function applyAppearance(appearance: ResolvedAppearance): void {
  document.documentElement.setAttribute('data-dg-appearance', appearance)
}

/**
 * Subscribe to macOS flipping appearance while the app is open. Returns an
 * unsubscribe function.
 *
 * This matters more here than on the web: Daygo is a long-lived background
 * agent whose window can stay open across a scheduled light/dark switch, so
 * "system" has to keep tracking rather than latch whatever was true at launch.
 */
export function watchSystemAppearance(
  onChange: (appearance: ResolvedAppearance) => void,
): () => void {
  const query = darkMediaQuery()
  if (!query) return () => {}

  const handler = (event: MediaQueryListEvent): void => {
    onChange(event.matches ? 'dark' : 'light')
  }
  query.addEventListener('change', handler)
  return () => query.removeEventListener('change', handler)
}
