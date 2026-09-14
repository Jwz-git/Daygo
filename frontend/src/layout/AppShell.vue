<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { i18n } from '@/i18n'
import { prefetchInstalledApplications } from '@/api/application'
import { usePointerHighlight } from '@/lib/pointerHighlight'
import { useRefractionFilter } from '@/lib/refractionFilter'
import { useRecordingStore } from '@/stores/recording'
import SideRail from './SideRail.vue'

const recording = useRecordingStore()
const shellRef = ref<HTMLElement | null>(null)
const pointer = usePointerHighlight(shellRef)
useRefractionFilter('subtle')

onMounted(() => {
  recording.startListening()
  // Warm the privacy grid's enumeration and icon caches while the user does
  // something else, so opening 设置 → 隐私 renders instantly. Idle-ish timing
  // keeps it out of the startup critical path.
  window.setTimeout(() => prefetchInstalledApplications(i18n.global.locale.value), 1500)
  // Mark the shell root as a tracking surface so the pointer-following
  // specular highlight reads across the panel. Children inherit it; the
  // highlight mask stays under each child's bounds automatically.
  if (shellRef.value) pointer.register(shellRef.value)
})
onBeforeUnmount(() => {
  recording.stopListening()
  if (shellRef.value) pointer.unregister(shellRef.value)
})

defineExpose({ registerPointer: pointer.register, unregisterPointer: pointer.unregister })
</script>

<template>
  <div ref="shellRef" class="shell">
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
