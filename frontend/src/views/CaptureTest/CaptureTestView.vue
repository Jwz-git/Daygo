<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import MacCaptureTestPanel from './MacCaptureTestPanel.vue'
import WindowsRecorderTestPanel from './WindowsRecorderTestPanel.vue'

type TestPlatform = 'macos' | 'windows'

const { t } = useI18n()
const platform = ref<TestPlatform>(navigator.userAgent.includes('Windows') ? 'windows' : 'macos')
</script>

<template>
  <div class="page capture-test">
    <PageHeader :title="t('captureTest.title')">
      <template #lead>
        <span class="dg-chip dg-chip--filled">{{ t('captureTest.badge') }}</span>
      </template>
    </PageHeader>

    <div class="content dg-scroll">
      <div class="platform-switch" role="tablist" :aria-label="t('captureTest.platformSwitch')">
        <button
          type="button"
          role="tab"
          class="platform-tab"
          :class="{ active: platform === 'macos' }"
          :aria-selected="platform === 'macos'"
          @click="platform = 'macos'"
        >
          {{ t('captureTest.macosTab') }}
        </button>
        <button
          type="button"
          role="tab"
          class="platform-tab"
          :class="{ active: platform === 'windows' }"
          :aria-selected="platform === 'windows'"
          @click="platform = 'windows'"
        >
          {{ t('captureTest.windowsTab') }}
        </button>
      </div>

      <MacCaptureTestPanel v-if="platform === 'macos'" />
      <WindowsRecorderTestPanel v-else />
    </div>
  </div>
</template>

<style scoped>
.content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

.platform-switch {
  display: inline-flex;
  align-self: flex-start;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--dg-panel-border);
  border-radius: 12px;
  background: var(--dg-track-fill);
}

.platform-tab {
  padding: 7px 14px;
  border: 0;
  border-radius: 8px;
  color: var(--dg-text-secondary);
  background: transparent;
  cursor: pointer;
}

.platform-tab.active {
  color: var(--dg-text-primary);
  background: var(--dg-panel-fill);
  box-shadow: var(--dg-panel-shadow);
}
</style>
