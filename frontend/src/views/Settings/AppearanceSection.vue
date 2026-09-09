<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import MilestoneNotice from '@/components/MilestoneNotice.vue'
import type { AppTheme } from '@/api/dto'
import { SYSTEM_LANGUAGE, type LanguagePreference } from '@/i18n/locales'
import { useAppearanceStore } from '@/stores/appearance'

import SettingRow from './SettingRow.vue'

const { t } = useI18n()
const appearance = useAppearanceStore()

/**
 * Endonyms, deliberately not translated: someone looking for their language
 * has to recognise it in a UI they cannot currently read.
 */
const LANGUAGE_LABELS: Record<Exclude<LanguagePreference, ''>, string> = {
  'zh-CN': '简体中文',
  en: 'English',
}

function themeLabel(theme: AppTheme): string {
  return t(`settings.appearance.themeOption.${theme}`)
}

function languageLabel(preference: LanguagePreference): string {
  return preference === SYSTEM_LANGUAGE
    ? t('settings.language.followSystem')
    : LANGUAGE_LABELS[preference]
}

function onThemeChange(event: Event): void {
  const value = (event.target as HTMLSelectElement).value as AppTheme
  void appearance.setTheme(value)
}

function onLanguageChange(event: Event): void {
  const value = (event.target as HTMLSelectElement).value as LanguagePreference
  void appearance.setLanguage(value)
}
</script>

<template>
  <SettingRow
    :title="t('settings.appearance.theme')"
    :hint="t('settings.appearance.themeDescription')"
  >
    <select
      class="dg-input select"
      :value="appearance.theme"
      :aria-label="t('settings.appearance.theme')"
      @change="onThemeChange"
    >
      <option v-for="option in appearance.themes" :key="option" :value="option">
        {{ themeLabel(option) }}
      </option>
    </select>
  </SettingRow>

  <SettingRow
    :title="t('settings.language.interface')"
    :hint="t('settings.language.interfaceDescription')"
  >
    <select
      class="dg-input select"
      :value="appearance.language"
      :aria-label="t('settings.language.interface')"
      @change="onLanguageChange"
    >
      <option v-for="option in appearance.languages" :key="option" :value="option">
        {{ languageLabel(option) }}
      </option>
    </select>
  </SettingRow>

  <!--
    Distinct from the interface language: this is the language the model writes
    card titles and summaries in. It is not part of SettingsDTO yet.
  -->
  <MilestoneNotice
    title-key="settings.language.output"
    description-key="settings.language.outputDescription"
    :milestone="2"
  />
</template>

<style scoped>
/*
 * SettingRow keeps its control column at content width; without a floor the two
 * selects would size to their shortest option and jump when the locale changes.
 */
.select {
  min-width: 168px;
}
</style>
