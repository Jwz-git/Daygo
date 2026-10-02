<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { AppTheme } from '@/api/dto'
import { SYSTEM_LANGUAGE, type LanguagePreference } from '@/i18n/locales'
import { useAppearanceStore } from '@/stores/appearance'
import { useTestToolsStore } from '@/stores/testTools'

import SettingRow from './SettingRow.vue'
import SwitchControl from './SwitchControl.vue'
import { useSettingsSection } from './useSettingsSection'

const { t } = useI18n()
const appearance = useAppearanceStore()
const testTools = useTestToolsStore()
const testToolsFailed = ref(false)

// The toggle only reveals a page that ships in dev/opt-in builds; hide it in
// the production installer so there is no dead switch behind an absent page.
const testToolsConfigurable = __DAYGO_TEST_TOOLS__

const { state: systemState, settings: systemSettings, load: loadSystem, persist: persistSystem, writeFailed: systemWriteFailed } =
  useSettingsSection()
const launchAtLogin = computed(() => systemSettings.value?.system.launchAtLogin ?? false)
const showDockIcon = computed(() => systemSettings.value?.system.showDockIcon ?? true)
const journalReminderEnabled = computed(
  () => systemSettings.value?.notifications.journalReminderEnabled ?? false,
)
const journalReminderTime = computed(
  () => systemSettings.value?.notifications.journalReminderTime ?? '18:00',
)
const dockConfigurable = document.documentElement.dataset.dgPlatform === 'darwin'

onMounted(() => void loadSystem())

function onToggleLaunchAtLogin(next: boolean): void {
  void persistSystem({ launchAtLogin: next })
}

function onToggleJournalReminder(next: boolean): void {
  void persistSystem({ journalReminderEnabled: next })
}

/**
 * A half-typed <input type="time"> reports an empty value; persisting that would
 * be rejected by the backend's normalizer and silently fall back to the default,
 * so an empty intermediate value is ignored and the previous time stands.
 */
function onJournalReminderTimeChange(event: Event): void {
  const value = (event.target as HTMLInputElement).value
  if (value === '') return
  void persistSystem({ journalReminderTime: value })
}

function onToggleTestTools(next: boolean): void {
  testToolsFailed.value = false
  void testTools.setEnabled(next).then((succeeded) => { testToolsFailed.value = !succeeded })
}

/**
 * Endonyms, deliberately not translated: someone looking for their language
 * has to recognise it in a UI they cannot currently read. Order follows
 * SUPPORTED_LOCALES so the list is not alphabetical in any one language.
 */
const LANGUAGE_LABELS: Record<Exclude<LanguagePreference, ''>, string> = {
  'zh-CN': '简体中文',
  'zh-Hant': '繁體中文',
  en: 'English',
  ja: '日本語',
  ko: '한국어',
  de: 'Deutsch',
  fr: 'Français',
  es: 'Español',
  'pt-BR': 'Português (Brasil)',
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
    :hint="appearance.persistence === 'unavailable' ? t('settings.appearance.persistenceUnavailable') : t('settings.appearance.themeDescription')"
  >
    <select
      class="dg-input select"
      :value="appearance.theme"
      :disabled="appearance.persistence === 'unavailable' || appearance.saving"
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
      :disabled="appearance.persistence === 'unavailable' || appearance.saving"
      :aria-label="t('settings.language.interface')"
      @change="onLanguageChange"
    >
      <option v-for="option in appearance.languages" :key="option" :value="option">
        {{ languageLabel(option) }}
      </option>
    </select>
  </SettingRow>

  <SettingRow
    :title="t('settings.general.launchAtLogin')"
    :hint="t('settings.general.launchAtLoginHint')"
  >
    <SwitchControl
      :checked="launchAtLogin"
      :disabled="systemState !== 'ready'"
      :label="t('settings.general.launchAtLogin')"
      @toggle="onToggleLaunchAtLogin"
    />
  </SettingRow>
  <p v-if="systemWriteFailed" class="write-error" role="alert">{{ t('settings.general.writeError') }}</p>

  <SettingRow v-if="dockConfigurable" :title="t('settings.general.showDockIcon')" :hint="t('settings.general.showDockIconHint')">
    <SwitchControl
      :checked="showDockIcon"
      :disabled="systemState !== 'ready'"
      :label="t('settings.general.showDockIcon')"
      @toggle="(next: boolean) => persistSystem({ showDockIcon: next })"
    />
  </SettingRow>

  <SettingRow
    :title="t('settings.general.journalReminder')"
    :hint="t('settings.general.journalReminderHint')"
  >
    <div class="reminder-control">
      <input
        v-if="journalReminderEnabled"
        class="dg-input reminder-time"
        type="time"
        :value="journalReminderTime"
        :disabled="systemState !== 'ready'"
        :aria-label="t('settings.general.journalReminderTime')"
        @change="onJournalReminderTimeChange"
      >
      <SwitchControl
        :checked="journalReminderEnabled"
        :disabled="systemState !== 'ready'"
        :label="t('settings.general.journalReminder')"
        @toggle="onToggleJournalReminder"
      />
    </div>
  </SettingRow>

  <template v-if="testToolsConfigurable">
    <SettingRow :title="t('settings.general.testTools')" :hint="t('settings.general.testToolsHint')">
      <SwitchControl
        :checked="testTools.enabled"
        :disabled="!testTools.loaded"
        :label="t('settings.general.testTools')"
        @toggle="onToggleTestTools"
      />
    </SettingRow>
    <p v-if="testToolsFailed" class="write-error" role="alert">{{ t('settings.general.writeError') }}</p>
  </template>
</template>

<style scoped>
/*
 * SettingRow keeps its control column at content width; without a floor the two
 * selects would size to their shortest option and jump when the locale changes.
 */
.select {
  min-width: 168px;
}

/* The time input only appears while the reminder is on; the switch keeps its
   place on the right so toggling does not shift the control column. */
.reminder-control {
  display: flex;
  align-items: center;
  gap: 12px;
}

.reminder-time {
  min-width: 112px;
  font-variant-numeric: tabular-nums;
}

.write-error {
  color: var(--dg-danger, #b42318);
  font-size: 13px;
}
</style>
