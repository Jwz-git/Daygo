import { ref, shallowRef } from 'vue'

import { WAILS_UNAVAILABLE, getSettings, updateSettings } from '@/api/settings'
import type { SettingsDTO, SettingsPatch } from '@/api/dto'

/*
 * Shared lifecycle of every settings section: load once, keep the effective
 * state in `settings`, and after a failed write re-read the authoritative
 * state instead of leaving an optimistic value on screen. Sections derive
 * their display values from `settings`; only in-progress input (a half-typed
 * field) stays in local refs.
 */

export type SettingsSectionState = 'loading' | 'ready' | 'unavailable' | 'error'

export function useSettingsSection() {
  const state = ref<SettingsSectionState>('loading')
  const settings = shallowRef<SettingsDTO | null>(null)

  async function load(): Promise<void> {
    try {
      settings.value = await getSettings()
      state.value = 'ready'
    } catch (error) {
      state.value =
        error instanceof Error && error.message === WAILS_UNAVAILABLE ? 'unavailable' : 'error'
    }
  }

  async function persist(patch: SettingsPatch): Promise<void> {
    try {
      settings.value = await updateSettings(patch)
      state.value = 'ready'
    } catch {
      try {
        settings.value = await getSettings()
        state.value = 'ready'
      } catch {
        state.value = 'error'
      }
    }
  }

  return { state, settings, load, persist }
}
