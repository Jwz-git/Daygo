<script setup lang="ts">
import { watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterView } from 'vue-router'

import { setNativeUiLabels } from '@/api/native'
import { setStatusItemLabels } from '@/api/recording'
import ErrorBoundary from '@/components/ErrorBoundary.vue'
import FatalErrorOverlay from '@/components/FatalErrorOverlay.vue'
import AppShell from '@/layout/AppShell.vue'
import { useTestToolsStore } from '@/stores/testTools'

// Root-level init: the shell never unmounts, so the subscription needs no teardown.
// Only wire up the test-tools store in builds that actually ship the test page.
const testTools = useTestToolsStore()
if (__DAYGO_TEST_TOOLS__) void testTools.initialize()

// Native surfaces (the menu-bar item, the application picker, the updater's
// install refusal) render outside the webview, so vue-i18n cannot reach them.
// Push the translated bundles to the backend on load and whenever the locale
// changes, so they follow the app's language. The shell never unmounts, so the
// watcher needs no teardown.
const { t, locale } = useI18n()
watch(
  locale,
  () => {
    void setStatusItemLabels({
      open: t('recording.menuBar.open'),
      recordings: t('recording.menuBar.recordings'),
      quit: t('recording.menuBar.quit'),
      pauseMenu: t('recording.menuBar.pauseMenu'),
      pause15: t('recording.menuBar.pause15'),
      pause30: t('recording.menuBar.pause30'),
      pause60: t('recording.menuBar.pause60'),
      pauseIndefinite: t('recording.menuBar.pauseIndefinite'),
      start: t('recording.menuBar.start'),
      resume: t('recording.menuBar.resume'),
      tooltip: t('recording.menuBar.tooltip'),
      titleRecording: t('recording.menuBar.titleRecording'),
      titlePaused: t('recording.menuBar.titlePaused'),
      titleIdle: t('recording.menuBar.titleIdle'),
    })
    void setNativeUiLabels({
      applicationPickerTitle: t('native.applicationPicker.title'),
      applicationPickerFilter: t('native.applicationPicker.filterExecutable'),
      updateOwnerRequired: t('native.updater.ownerRequired'),
    })
  },
  { immediate: true },
)
</script>

<template>
  <AppShell>
    <!--
      No page <Transition> here, deliberately. A `mode="out-in"` transition
      around lazily-loaded routes wedges: an interrupted leave can drop the
      incoming view, leaving <main> empty — a blank content area that persists
      across further navigation. Correctness beats a 200ms fade. The per-route
      key gives each route a clean instance; ErrorBoundary keeps a thrown page
      from tearing down the shell.
    -->
    <RouterView v-slot="{ Component, route }">
      <ErrorBoundary :key="route.path">
        <component :is="Component" />
      </ErrorBoundary>
    </RouterView>
  </AppShell>
  <FatalErrorOverlay />
</template>
