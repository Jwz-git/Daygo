<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import SettingRow from './SettingRow.vue'
import SwitchControl from './SwitchControl.vue'
import { useSettingsSection } from './useSettingsSection'

const { t } = useI18n()
const { state, settings, load, persist } = useSettingsSection()

const enabled = computed(() => settings.value?.system.agentEditsEnabled ?? false)

onMounted(() => void load())

function onToggle(next: boolean): void {
  void persist({ agentEditsEnabled: next })
}
</script>

<template>
  <SettingRow
    :title="t('settings.agentAccess.editsTitle')"
    :hint="t('settings.agentAccess.editsHint')"
  >
    <SwitchControl
      :checked="enabled"
      :disabled="state !== 'ready'"
      :label="t('settings.agentAccess.editsTitle')"
      @toggle="onToggle"
    />
  </SettingRow>
</template>
