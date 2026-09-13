import { defineStore } from 'pinia'
import { ref } from 'vue'

import { getSettings, onSettingsChanged, updateSettings } from '@/api/settings'

/** The storage key behind the toggle; settings:changed events carry these. */
const TEST_TOOLS_KEY = 'system.testToolsEnabled'

/*
 * Cross-page state for the test-tools gate. The side rail, the settings toggle
 * and the test page itself all read it, so it lives here rather than in the
 * settings section that flips it. In the Wails app the settings:changed event
 * keeps the value in step; in a browser preview the toggle writes through
 * setEnabled directly.
 */
export const useTestToolsStore = defineStore('testTools', () => {
  const enabled = ref(false)
  const loaded = ref(false)
  let stopEvents: (() => void) | null = null
  let initialLoad: Promise<void> | null = null

  async function refresh(): Promise<void> {
    try {
      enabled.value = (await getSettings()).system.testToolsEnabled
    } catch {
      // No database (or no bridge and no fixture): the page stays hidden.
      enabled.value = false
    }
    loaded.value = true
  }

  function initialize(): Promise<void> {
    if (initialLoad !== null) return initialLoad
    stopEvents = onSettingsChanged((keys) => {
      if (keys.includes(TEST_TOOLS_KEY)) void refresh()
    })
    initialLoad = refresh()
    return initialLoad
  }

  async function setEnabled(next: boolean): Promise<boolean> {
    try {
      const settings = await updateSettings({ testToolsEnabled: next })
      enabled.value = settings.system.testToolsEnabled
      return true
    } catch {
      return false
    }
  }

  return { enabled, loaded, initialize, setEnabled, refresh, stopEvents }
})
