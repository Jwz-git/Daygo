<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  checkForUpdates,
  getUpdaterState,
  onUpdateAvailable,
  setAutomaticUpdateChecks,
  type UpdaterState,
} from '@/api/update'
import SettingRow from './SettingRow.vue'
import SwitchControl from './SwitchControl.vue'

const { t, d } = useI18n()
const state = ref<UpdaterState | null>(null)
const loading = ref(true)
const checking = ref(false)
const unavailable = ref(false)
const failed = ref(false)
let stopEvents: () => void = () => undefined
let pollTimer: ReturnType<typeof setTimeout> | null = null

const lastChecked = computed(() => state.value?.lastCheckedAtTs == null
  ? t('settings.update.never')
  : d(new Date(state.value.lastCheckedAtTs * 1000), 'short'))

async function load(): Promise<void> {
  loading.value = true
  try {
    state.value = await getUpdaterState()
    unavailable.value = false
    if (state.value.checking) schedulePoll()
  } catch {
    unavailable.value = true
  } finally {
    loading.value = false
  }
}

function schedulePoll(): void {
  if (pollTimer !== null) return
  pollTimer = setTimeout(() => {
    pollTimer = null
    void load()
  }, 1000)
}

async function check(): Promise<void> {
  checking.value = true
  failed.value = false
  try {
    await checkForUpdates(true)
    await load()
  } catch {
    failed.value = true
  } finally {
    checking.value = false
  }
}

async function setAutomatic(enabled: boolean): Promise<void> {
  failed.value = false
  try {
    await setAutomaticUpdateChecks(enabled)
    await load()
  } catch {
    failed.value = true
    await load()
  }
}

onMounted(() => {
  stopEvents = onUpdateAvailable((next) => { state.value = next })
  void load()
})
onBeforeUnmount(() => {
  stopEvents()
  if (pollTimer !== null) clearTimeout(pollTimer)
})
</script>

<template>
  <SettingGroup :title="t('settings.update.title')" :hint="t('settings.update.hint')">
    <template v-if="!unavailable">
      <SettingRow :title="t('settings.update.automatic')" :hint="t('settings.update.automaticHint')">
        <SwitchControl
          :checked="state?.automatic ?? false"
          :disabled="loading"
          :label="t('settings.update.automatic')"
          @toggle="setAutomatic"
        />
      </SettingRow>
      <SettingRow :title="t('settings.update.check')" :hint="t('settings.update.lastChecked', { value: lastChecked })">
        <button class="dg-button" type="button" :disabled="loading || checking || state?.checking" @click="check">
          {{ checking || state?.checking ? t('settings.update.checking') : t('settings.update.check') }}
        </button>
      </SettingRow>
      <p v-if="state?.availableVersion !== undefined && state?.availableVersion !== null" class="notice" role="status">
        {{ state.availableVersion === ''
          ? t('settings.update.availableUnknownVersion')
          : t('settings.update.available', { version: state.availableVersion }) }}
      </p>
    </template>
    <p v-else class="muted">{{ t('settings.update.unavailable') }}</p>
    <p v-if="failed" class="error" role="alert">{{ t('settings.update.failed') }}</p>
  </SettingGroup>
</template>

<style scoped>
.notice { color: var(--dg-text-primary); font-size: 13px; }
.muted { color: var(--dg-text-secondary); font-size: 13px; }
.error { color: var(--dg-danger); font-size: 13px; }
</style>
