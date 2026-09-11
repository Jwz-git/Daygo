<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { WAILS_UNAVAILABLE, getSettings, updateSettings } from '@/api/settings'

import SettingRow from './SettingRow.vue'

const { t } = useI18n()

type LoadState = 'loading' | 'ready' | 'unavailable' | 'error'

const enabled = ref(false)
const state = ref<LoadState>('loading')

const hint = computed(() =>
  state.value === 'unavailable'
    ? t('settings.recognition.unavailable')
    : t('settings.recognition.hint'),
)

onMounted(async () => {
  try {
    const settings = await getSettings()
    enabled.value = settings.llm.recognitionEnhancementEnabled
    state.value = 'ready'
  } catch (error) {
    state.value =
      error instanceof Error && error.message === WAILS_UNAVAILABLE ? 'unavailable' : 'error'
  }
})

async function onToggle(event: Event): Promise<void> {
  const next = (event.target as HTMLInputElement).checked
  try {
    const settings = await updateSettings({ recognitionEnhancementEnabled: next })
    enabled.value = settings.llm.recognitionEnhancementEnabled
  } catch {
    // A failed write must not leave an optimistic value on screen. Re-read
    // the authoritative state instead of keeping the toggle where the user
    // left it.
    try {
      const settings = await getSettings()
      enabled.value = settings.llm.recognitionEnhancementEnabled
    } catch {
      state.value = 'error'
    }
  }
}
</script>

<template>
  <SettingRow :title="t('settings.recognition.title')" :hint="hint">
    <label class="switch">
      <input
        type="checkbox"
        role="switch"
        class="switch__input"
        :checked="enabled"
        :disabled="state !== 'ready'"
        :aria-label="t('settings.recognition.title')"
        @change="onToggle"
      >
      <span class="switch__track" aria-hidden="true" />
    </label>
  </SettingRow>
</template>

<style scoped>
.switch {
  display: inline-flex;
  align-items: center;
  padding: 9px 0;
}

.switch__input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: 0;
  opacity: 0;
}

.switch__track {
  position: relative;
  display: inline-block;
  box-sizing: border-box;
  width: 40px;
  height: 22px;
  border: 1px solid var(--dg-input-border);
  border-radius: 999px;
  background: var(--dg-input-fill);
  cursor: pointer;
  transition:
    background var(--dg-motion-fast) ease,
    border-color var(--dg-motion-fast) ease;
}

.switch__track::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  box-sizing: border-box;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--dg-text-secondary);
  transition:
    transform var(--dg-motion-fast) var(--dg-ease-out),
    background var(--dg-motion-fast) ease;
}

.switch__input:checked + .switch__track {
  border-color: var(--dg-accent);
  background: var(--dg-control-fill);
}

.switch__input:checked + .switch__track::after {
  transform: translateX(18px);
  background: var(--dg-accent-strong);
}

.switch__input:focus-visible + .switch__track {
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.switch__input:disabled + .switch__track {
  opacity: 0.5;
  cursor: not-allowed;
}

@media (prefers-reduced-motion: reduce) {
  .switch__track,
  .switch__track::after {
    transition: none;
  }
}
</style>
