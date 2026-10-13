<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { openProjectSupport, PROJECT_SUPPORT_URL } from '@/api/projectSupport'
import SettingGroup from './SettingGroup.vue'

const { t } = useI18n()

function onSupportClick(event: MouseEvent): void {
  if (openProjectSupport()) event.preventDefault()
}
</script>

<template>
  <SettingGroup :title="t('settings.support.title')" :hint="t('settings.support.hint')">
    <a
      class="dg-button support-link"
      :href="PROJECT_SUPPORT_URL"
      target="_blank"
      rel="noopener noreferrer"
      @click="onSupportClick"
    >
      <span aria-hidden="true">☆</span>
      {{ t('settings.support.action') }}
    </a>
  </SettingGroup>
</template>

<style scoped>
.support-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  text-decoration: none;
}
</style>
