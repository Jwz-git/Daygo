<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { CAPTURE_HEIGHTS, CAPTURE_INTERVAL_SECONDS } from '@/api/dto'
import { getRecordingDirectory } from '@/api/recording'
import SettingRow from './SettingRow.vue'
import { useSettingsSection } from './useSettingsSection'

const BYTES_PER_GB = 1024 ** 3

const { t } = useI18n()
const { state, settings, load, persist, writeFailed } = useSettingsSection()

const intervalSeconds = computed(
  () => settings.value?.capture.intervalSeconds ?? CAPTURE_INTERVAL_SECONDS[2],
)
const captureHeight = computed(() => settings.value?.capture.captureHeight ?? CAPTURE_HEIGHTS[1])

const limitBytes = computed(() => settings.value?.storage.recordingsLimitBytes ?? 0)
const unlimited = computed(() => limitBytes.value === 0)
/** Shown in the GB field when a limit exists; 1 while unlimited. */
const limitGb = computed(() => {
  const bytes = limitBytes.value
  return bytes === 0 ? 1 : bytes / BYTES_PER_GB
})

const recordingDirectory = ref('')
const gbInput = ref('1')
function syncGbInput(): void { gbInput.value = String(limitGb.value) }
onMounted(() => { void load(); void getRecordingDirectory().then((value) => { recordingDirectory.value = value }).catch(() => undefined) })

function onIntervalChange(event: Event): void {
  void persist({ intervalSeconds: Number((event.target as HTMLSelectElement).value) })
}

function onHeightChange(event: Event): void {
  void persist({ captureHeight: Number((event.target as HTMLSelectElement).value) })
}

function onUnlimitedToggle(event: Event): void {
  const checked = (event.target as HTMLInputElement).checked
  if (checked) {
    void persist({ recordingsLimitBytes: 0 })
    return
  }
  const gb = Math.round(parseFloat(gbInput.value))
  void persist({ recordingsLimitBytes: Number.isFinite(gb) && gb >= 1 ? gb * BYTES_PER_GB : BYTES_PER_GB })
}

function onLimitChange(event: Event): void {
  const gb = Math.round(parseFloat((event.target as HTMLInputElement).value))
  if (!Number.isFinite(gb) || gb < 1) {
    // Invalid input reverts to the authoritative value on the next render.
    return
  }
  void persist({ recordingsLimitBytes: gb * BYTES_PER_GB })
}
</script>

<template>
  <SettingRow
    :title="t('settings.storage.interval')"
    :hint="t('settings.storage.intervalHint')"
  >
    <select
      class="dg-input select"
      :value="intervalSeconds"
      :disabled="state !== 'ready'"
      :aria-label="t('settings.storage.interval')"
      @change="onIntervalChange"
    >
      <option v-for="seconds in CAPTURE_INTERVAL_SECONDS" :key="seconds" :value="seconds">
        {{ t('settings.storage.intervalOption', { seconds }) }}
      </option>
    </select>
  </SettingRow>

  <SettingRow
    :title="t('settings.storage.height')"
    :hint="t('settings.storage.heightHint')"
  >
    <select
      class="dg-input select"
      :value="captureHeight"
      :disabled="state !== 'ready'"
      :aria-label="t('settings.storage.height')"
      @change="onHeightChange"
    >
      <option v-for="height in CAPTURE_HEIGHTS" :key="height" :value="height">
        {{ t(`settings.storage.heightOption.${height}`) }}
      </option>
    </select>
  </SettingRow>

  <SettingRow
    :title="t('settings.storage.limit')"
    :hint="t('settings.storage.limitHint')"
  >
    <div class="limit">
      <label class="limit__unlimited">
        <input
          type="checkbox"
          :checked="unlimited"
          :disabled="state !== 'ready'"
          @change="onUnlimitedToggle"
        >
        {{ t('settings.storage.unlimited') }}
      </label>
      <input
        ref="gbInput"
        class="dg-input limit__value"
        type="number"
        min="1"
        step="1"
        :value="limitGb"
        :disabled="unlimited || state !== 'ready'"
        :aria-label="t('settings.storage.limit')"
        @change="onLimitChange"
      >
      <span class="limit__unit">{{ t('settings.storage.unitGb') }}</span>
    </div>
  </SettingRow>
  <SettingRow :title="t('settings.storage.directory')" :hint="t('settings.storage.directoryHint')">
    <code class="directory">{{ recordingDirectory || t('settings.storage.directoryUnavailable') }}</code>
  </SettingRow>
  <p v-if="writeFailed" class="write-error" role="alert">{{ t('settings.storage.writeError') }}</p>
</template>

<style scoped>
.select {
  min-width: 128px;
}

.limit {
  display: flex;
  align-items: center;
  gap: 10px;
}

.limit__unlimited {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-text-primary);
  font-size: 13px;
  cursor: pointer;
}

.limit__unlimited:has(input:disabled) {
  cursor: not-allowed;
}

.limit__value {
  width: 88px;
  text-align: right;
}

.limit__unit {
  color: var(--dg-text-secondary);
  font-size: 13px;
}
.write-error {
  color: var(--dg-danger, #b42318);
  font-size: 13px;
}

@media (max-width: 620px) {
  .limit {
    justify-content: space-between;
  }
}
</style>
