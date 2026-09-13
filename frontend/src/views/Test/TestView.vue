<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import PageHeader from '@/components/PageHeader.vue'
import { clearHistoryData, getTimelineActionAvailability, getTimelineCapabilities } from '@/api/timeline'
import { useTestToolsStore } from '@/stores/testTools'
import MacCaptureTestPanel from './MacCaptureTestPanel.vue'
import WindowsRecorderTestPanel from './WindowsRecorderTestPanel.vue'

type TestPlatform = 'macos' | 'windows'

const { t } = useI18n()
const router = useRouter()
const testTools = useTestToolsStore()

// The rail hides this page while the toggle is off; a stale #/test hash or
// turning the toggle off while already here must not leave the page reachable.
watch(
  [() => testTools.loaded, () => testTools.enabled],
  ([loaded, enabled]) => {
    if (loaded && !enabled) void router.replace({ name: 'timeline' })
  },
  { immediate: true },
)

const platform = ref<TestPlatform>(navigator.userAgent.includes('Windows') ? 'windows' : 'macos')

const actionBindings = getTimelineActionAvailability()
const canWrite = ref(false)
const confirmingClear = ref(false)
const clearing = ref(false)
const clearFailed = ref(false)
const canClear = computed(() => canWrite.value && actionBindings.clearHistory)

void getTimelineCapabilities()
  .then((capabilities) => { canWrite.value = capabilities?.canWrite ?? false })
  .catch(() => undefined)

async function clearHistory(): Promise<void> {
  clearing.value = true
  clearFailed.value = false
  try {
    await clearHistoryData()
    confirmingClear.value = false
  } catch {
    clearFailed.value = true
  } finally {
    clearing.value = false
  }
}
</script>

<template>
  <div class="page test-page">
    <PageHeader :title="t('test.title')">
      <template #lead>
        <span class="dg-chip dg-chip--filled">{{ t('test.badge') }}</span>
      </template>
    </PageHeader>

    <div class="content dg-scroll">
      <section class="test__section">
        <div class="section-heading">
          <h2>{{ t('test.capture.title') }}</h2>
        </div>
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
      </section>

      <section class="test__section">
        <div class="section-heading">
          <h2>{{ t('test.clear.title') }}</h2>
        </div>
        <div class="tool-row">
          <template v-if="confirmingClear">
            <span class="tool-row__confirm">{{ t('test.clear.confirm') }}</span>
            <button type="button" class="dg-button" :disabled="clearing" @click="confirmingClear = false">
              {{ t('common.action.cancel') }}
            </button>
            <button type="button" class="dg-button tool-row__danger" :disabled="clearing" @click="clearHistory">
              {{ clearing ? t('test.clear.pending') : t('common.action.delete') }}
            </button>
          </template>
          <button
            v-else
            type="button"
            class="dg-button tool-row__danger"
            :disabled="!canClear"
            :title="canClear ? t('test.clear.action') : t('test.clear.unavailable')"
            @click="confirmingClear = true"
          >
            {{ t('test.clear.action') }}
          </button>
        </div>
        <p class="tool-note">{{ t('test.clear.note') }}</p>
        <p v-if="clearFailed" class="tool-error" role="alert">{{ t('test.clear.failed') }}</p>
      </section>
    </div>
  </div>
</template>

<style scoped>
.content {
  display: flex;
  flex-direction: column;
  gap: 22px;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

.test__section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: var(--dg-settings-content-max);
}

.section-heading h2 {
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 650;
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

.tool-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.tool-row__confirm { color: var(--dg-text-secondary); font-size: 11px; }
.tool-row__danger { color: var(--dg-danger); }

.tool-row__danger:not(:disabled):hover {
  background: color-mix(in srgb, var(--dg-danger) 9%, transparent);
}

.tool-note { color: var(--dg-text-muted); font-size: 10px; }
.tool-error { color: var(--dg-danger); font-size: 11px; }
</style>
