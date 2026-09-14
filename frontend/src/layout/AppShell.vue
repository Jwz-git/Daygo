<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { i18n } from '@/i18n'
import { prefetchInstalledApplications } from '@/api/application'
import { useRecordingStore } from '@/stores/recording'
import SideRail from './SideRail.vue'

const recording = useRecordingStore()
onMounted(() => {
  recording.startListening()
  // Warm the privacy grid's enumeration and icon caches while the user does
  // something else, so opening 设置 → 隐私 renders instantly. Idle-ish timing
  // keeps it out of the startup critical path.
  window.setTimeout(() => prefetchInstalledApplications(i18n.global.locale.value), 1500)
})
onBeforeUnmount(() => recording.stopListening())
</script>

<template>
  <div class="shell">
    <SideRail />

    <main class="panel dg-panel">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.shell {
  position: relative;
  display: grid;
  grid-template-columns: var(--dg-rail-width) minmax(0, 1fr);
  height: 100vh;
  padding: 0;
  overflow: hidden;
  /* The tonal field is what makes the panel's backdrop blur read as glass; a
     flat colour behind it would only show transparent gray. The gradient is
     broad and slow on purpose — no detail that competes with content. */
  background-color: var(--dg-window-bg);
  background-image: var(--dg-window-gradient);
}

.panel {
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  min-width: 0;
  margin: var(--dg-window-padding) var(--dg-window-padding)
    var(--dg-window-padding) 0;
  overflow: hidden;
}
</style>
