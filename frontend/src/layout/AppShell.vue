<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { useRefractionFilter } from '@/lib/refractionFilter'
import { useRecordingStore } from '@/stores/recording'
import SideRail from './SideRail.vue'
import WindowsTitleBar from './WindowsTitleBar.vue'

const recording = useRecordingStore()
useRefractionFilter('subtle')

onMounted(() => recording.startListening())
onBeforeUnmount(() => recording.stopListening())

const isWindows = document.documentElement.dataset.dgPlatform === 'windows'
</script>

<template>
  <div class="app-frame">
    <WindowsTitleBar v-if="isWindows" />
    <div class="shell">
      <SideRail />

      <main class="panel dg-panel">
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
.app-frame {
  height: 100vh;
  overflow: hidden;
  background-color: var(--dg-window-bg);
  background-image: var(--dg-window-gradient);
}

.shell {
  position: relative;
  display: grid;
  grid-template-columns: var(--dg-rail-width) minmax(0, 1fr);
  height: 100%;
  padding: 0;
  overflow: hidden;
  /* The tonal field is what makes the panel's backdrop blur read as glass; a
     flat colour behind it would only show transparent gray. The gradient is
     broad and slow on purpose — no detail that competes with content. */
  background-color: var(--dg-window-bg);
  background-image: var(--dg-window-gradient);
}

:root[data-dg-platform='windows'] .shell {
  height: calc(100% - var(--dg-windows-titlebar-height));
  background: transparent;
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
