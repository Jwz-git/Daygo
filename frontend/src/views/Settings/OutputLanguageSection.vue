<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  WAILS_UNAVAILABLE,
  getSettings,
  onSettingsChanged,
  updateSettings,
} from '@/api/settings'

import SettingRow from './SettingRow.vue'

type LoadState = 'loading' | 'ready' | 'unavailable' | 'error'

const { t } = useI18n()
const state = ref<LoadState>('loading')
const saved = ref('')
const draft = ref('')
const saving = ref(false)
const saveFailed = ref(false)
let stopSettingsEvents: (() => void) | null = null

const changed = computed(() => draft.value.trim() !== saved.value)

async function load(): Promise<void> {
  try {
    const settings = await getSettings()
    saved.value = settings.llm.outputLanguage
    draft.value = settings.llm.outputLanguage
    state.value = 'ready'
  } catch (error) {
    state.value =
      error instanceof Error && error.message === WAILS_UNAVAILABLE
        ? 'unavailable'
        : 'error'
  }
}

async function save(value: string): Promise<void> {
  if (state.value !== 'ready' || saving.value) return
  saving.value = true
  saveFailed.value = false
  try {
    const settings = await updateSettings({ outputLanguage: value.trim() })
    saved.value = settings.llm.outputLanguage
    draft.value = settings.llm.outputLanguage
  } catch {
    saveFailed.value = true
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
  stopSettingsEvents = onSettingsChanged((keys) => {
    if (keys.includes('llm.outputLanguage')) void load()
  })
})

onBeforeUnmount(() => stopSettingsEvents?.())
</script>

<template>
  <SettingRow
    :title="t('settings.language.output')"
    :hint="state === 'unavailable' ? t('settings.language.outputUnavailable') : t('settings.language.outputDescription')"
  >
    <div class="language-control">
      <input
        v-model="draft"
        class="dg-input language-input"
        type="text"
        :placeholder="t('settings.language.outputPlaceholder')"
        :disabled="state !== 'ready' || saving"
        :aria-label="t('settings.language.output')"
        @keydown.enter="save(draft)"
      >
      <button
        type="button"
        class="dg-button dg-button--primary"
        :disabled="!changed || state !== 'ready' || saving"
        @click="save(draft)"
      >
        {{ saving ? t('settings.language.saving') : t('common.action.save') }}
      </button>
      <button
        type="button"
        class="dg-button"
        :disabled="saved === '' || state !== 'ready' || saving"
        @click="save('')"
      >
        {{ t('common.action.reset') }}
      </button>
    </div>
    <p v-if="saveFailed" class="save-error" role="alert">
      {{ t('settings.language.outputError') }}
    </p>
  </SettingRow>
</template>

<style scoped>
.language-control { display: flex; align-items: center; gap: 7px; }
.language-input { width: 170px; }
.save-error { margin-top: 6px; color: var(--dg-danger); font-size: 11px; text-align: right; }

@media (max-width: 620px) {
  .language-control { align-items: stretch; flex-wrap: wrap; }
  .language-input { width: 100%; }
}
</style>
