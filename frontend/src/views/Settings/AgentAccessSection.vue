<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import SettingRow from './SettingRow.vue'
import SwitchControl from './SwitchControl.vue'
import { useSettingsSection } from './useSettingsSection'

const { t } = useI18n()
const { state, settings, load, persist } = useSettingsSection()

const enabled = computed(() => settings.value?.system.agentEditsEnabled ?? false)
const chatEdits = computed(() => settings.value?.chat.editMode === 'edits')

onMounted(() => void load())

function onToggle(next: boolean): void {
  void persist({ agentEditsEnabled: next })
}

function onToggleChatEdits(next: boolean): void {
  void persist({ chatEditMode: next ? 'edits' : 'readonly' })
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
  <SettingRow
    :title="t('settings.agentAccess.chatEditsTitle')"
    :hint="t('settings.agentAccess.chatEditsHint')"
  >
    <SwitchControl
      :checked="chatEdits"
      :disabled="state !== 'ready'"
      :label="t('settings.agentAccess.chatEditsTitle')"
      @toggle="onToggleChatEdits"
    />
  </SettingRow>
</template>
