import { defineStore } from 'pinia'
import { ref } from 'vue'

import { getSettings, onSettingsChanged, updateSettings } from '@/api/settings'

/** The storage key behind the toggle; settings:changed events carry these. */
const SHOW_TOKEN_USAGE_KEY = 'llm.showTokenUsage'

/*
 * Cross-page state for the token-usage card gate. The daily view, the weekly
 * view and the settings toggle all read it, so it lives here rather than in the
 * settings section that flips it. In the Wails app the settings:changed event
 * keeps the value in step; in a browser preview the toggle writes through
 * setShowTokenUsage directly.
 *
 * The card is only hidden, never unfetched: GetTokenUsage stays callable and the
 * read stays the report's own concern.
 */
export const useTokenUsageVisibilityStore = defineStore('tokenUsageVisibility', () => {
  const showTokenUsage = ref(false)
  const loaded = ref(false)
  let stopEvents: (() => void) | null = null
  let initialLoad: Promise<void> | null = null

  async function refresh(): Promise<void> {
    try {
      showTokenUsage.value = (await getSettings()).llm.showTokenUsage
    } catch {
      // No database, or no bridge and no fixture: keep the default off, which
      // is also what a database that never wrote the key reports.
      showTokenUsage.value = false
    }
    loaded.value = true
  }

  function initialize(): Promise<void> {
    if (initialLoad !== null) return initialLoad
    stopEvents = onSettingsChanged((keys) => {
      if (keys.includes(SHOW_TOKEN_USAGE_KEY)) void refresh()
    })
    initialLoad = refresh()
    return initialLoad
  }

  async function setShowTokenUsage(next: boolean): Promise<boolean> {
    try {
      const settings = await updateSettings({ showTokenUsage: next })
      showTokenUsage.value = settings.llm.showTokenUsage
      return true
    } catch {
      return false
    }
  }

  return { showTokenUsage, loaded, initialize, setShowTokenUsage, refresh }
})
