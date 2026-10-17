import { defineStore } from 'pinia'
import { ref } from 'vue'

import { getCapabilities } from '@/api/capabilities'
import { getSettings, updateSettings } from '@/api/settings'

/*
 * First-run guide gate. The guide shows only on the writer instance and only
 * while system.onboardingCompleted is unset — the v25 migration marks it for
 * databases that were already in use, so upgrading users never see it. Any
 * read failure (plain browser, database error) resolves to "done": the guide
 * must never stand between the user and the app.
 */
export type OnboardingStatus = 'unknown' | 'pending' | 'done'

export const useOnboardingStore = defineStore('onboarding', () => {
  const status = ref<OnboardingStatus>('unknown')
  /*
   * Preview (opened from the test page by a user past their first run): the
   * guide shows, but the category preset is not written and completion is
   * not recorded — the setting is already set, and the preset binding would
   * refuse anyway. The other steps write settings as the Settings page does.
   */
  const preview = ref(false)

  async function initialize(): Promise<void> {
    if (status.value !== 'unknown') return
    try {
      const [settings, capabilities] = await Promise.all([getSettings(), getCapabilities()])
      status.value = !settings.system.onboardingCompleted && capabilities?.canWrite === true ? 'pending' : 'done'
    } catch {
      status.value = 'done'
    }
  }

  /**
   * Marks the guide finished. The overlay closes even when the write fails;
   * the setting stays unset then and the guide simply returns next launch.
   */
  async function complete(): Promise<void> {
    if (preview.value) {
      close()
      return
    }
    try {
      await updateSettings({ onboardingCompleted: true })
    } catch {
      // See above: a lost write only means the guide shows again.
    } finally {
      status.value = 'done'
    }
  }

  /** Persists completion but keeps the overlay up (its last step is still shown). */
  async function markCompleted(): Promise<boolean> {
    if (preview.value) return true
    try {
      await updateSettings({ onboardingCompleted: true })
      return true
    } catch {
      return false
    }
  }

  function close(): void {
    status.value = 'done'
    preview.value = false
  }

  function startPreview(): void {
    preview.value = true
    status.value = 'pending'
  }

  return { status, preview, initialize, complete, markCompleted, close, startPreview }
})
