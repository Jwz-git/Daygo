<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import { CAPTURE_INTERVAL_SECONDS, type AppTheme } from '@/api/dto'
import DgSelect, { type DgSelectOption } from '@/components/DgSelect.vue'
import { SYSTEM_LANGUAGE, type LanguagePreference } from '@/i18n/locales'
import { LANGUAGE_LABELS } from '@/lib/languageLabels'
import { useAppearanceStore } from '@/stores/appearance'
import SwitchControl from '@/views/Settings/SwitchControl.vue'
import { useSettingsSection } from '@/views/Settings/useSettingsSection'

import OnboardingStep from './OnboardingStep.vue'

/*
 * The settings a first run should see before recording: appearance, language,
 * launch at login and the capture interval. Each control writes through the
 * same stores and patch path as the Settings page, and shows the value the
 * backend returns.
 */
const emit = defineEmits<{ back: []; next: [] }>()
const { t } = useI18n()
const appearance = useAppearanceStore()
const { state, settings, load, persist, writeFailed } = useSettingsSection()

onMounted(() => { void load() })

const launchAtLogin = computed(() => settings.value?.system.launchAtLogin ?? false)
const intervalSeconds = computed(() => settings.value?.capture.intervalSeconds ?? CAPTURE_INTERVAL_SECONDS[2])

const themeOptions = computed<DgSelectOption<AppTheme>[]>(() =>
  appearance.themes.map((value) => ({ value, label: t(`settings.appearance.themeOption.${value}`) })),
)

const languageOptions = computed<DgSelectOption<LanguagePreference>[]>(() =>
  appearance.languages.map((value) => ({
    value,
    label: value === SYSTEM_LANGUAGE ? t('settings.language.followSystem') : LANGUAGE_LABELS[value],
  })),
)

const intervalOptions = computed<DgSelectOption<number>[]>(() =>
  CAPTURE_INTERVAL_SECONDS.map((seconds) => ({ value: seconds, label: t('settings.storage.intervalOption', { seconds }) })),
)
</script>

<template>
  <OnboardingStep :title="t('onboarding.preferences.title')" :body="t('onboarding.preferences.body')" @back="emit('back')">
    <div class="ob-prefs">
      <div class="ob-pref">
        <span class="ob-pref__text"><strong>{{ t('settings.appearance.theme') }}</strong></span>
        <DgSelect
          class="ob-pref__select"
          :model-value="appearance.theme"
          :options="themeOptions"
          :disabled="appearance.persistence === 'unavailable' || appearance.saving"
          :aria-label="t('settings.appearance.theme')"
          @update:model-value="(value) => void appearance.setTheme(value)"
        />
      </div>
      <div class="ob-pref">
        <span class="ob-pref__text"><strong>{{ t('settings.language.interface') }}</strong></span>
        <DgSelect
          class="ob-pref__select"
          :model-value="appearance.language"
          :options="languageOptions"
          :disabled="appearance.persistence === 'unavailable' || appearance.saving"
          :aria-label="t('settings.language.interface')"
          @update:model-value="(value) => void appearance.setLanguage(value)"
        />
      </div>
      <div class="ob-pref">
        <span class="ob-pref__text">
          <strong>{{ t('settings.storage.interval') }}</strong>
          <span>{{ t('settings.storage.intervalHint') }}</span>
        </span>
        <DgSelect
          class="ob-pref__select"
          :model-value="intervalSeconds"
          :options="intervalOptions"
          :disabled="state !== 'ready'"
          :aria-label="t('settings.storage.interval')"
          @update:model-value="(value) => void persist({ intervalSeconds: value })"
        />
      </div>
      <div class="ob-pref">
        <span class="ob-pref__text">
          <strong>{{ t('settings.general.launchAtLogin') }}</strong>
          <span>{{ t('settings.general.launchAtLoginHint') }}</span>
        </span>
        <SwitchControl
          :checked="launchAtLogin"
          :disabled="state !== 'ready'"
          :label="t('settings.general.launchAtLogin')"
          @toggle="(value) => void persist({ launchAtLogin: value })"
        />
      </div>
    </div>
    <p v-if="writeFailed" class="ob-error" role="alert">{{ t('onboarding.preferences.failed') }}</p>

    <template #actions>
      <button type="button" class="dg-button dg-button--lg dg-button--primary ob-primary" @click="emit('next')">{{ t('onboarding.next') }}</button>
    </template>
  </OnboardingStep>
</template>

<style scoped>
.ob-prefs {
  display: grid;
  max-width: 560px;
  margin: 0 auto;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 14px;
  background: var(--dg-timeline-card-fill);
}

.ob-pref {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 56px;
  padding: 10px 16px;
}

.ob-pref + .ob-pref { border-top: 1px solid var(--dg-timeline-grid); }

.ob-pref__text {
  display: grid;
  flex: 1;
  gap: 2px;
  min-width: 0;
}

.ob-pref__text strong { color: var(--dg-text-primary); font-size: 13px; font-weight: 600; }
.ob-pref__text span { color: var(--dg-text-muted); font-size: 11px; line-height: 1.45; }

.ob-pref__select { width: 180px; flex: none; }

.ob-error {
  margin: 14px 0 0;
  color: var(--dg-danger);
  font-size: 12px;
  text-align: center;
}
</style>
