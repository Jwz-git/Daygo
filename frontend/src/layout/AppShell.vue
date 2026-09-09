<script setup lang="ts">
import SideRail from './SideRail.vue'
</script>

<template>
  <div class="shell">
    <!--
      Noise texture carried over from the previous single-file shell: it is a
      cheap grain layer and is independent of the palette.
    -->
    <div class="window-grain" aria-hidden="true"></div>

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
  /* Legacy MainView pads top/trailing/bottom by 15; the rail sits flush left. */
  padding: var(--dg-window-padding) var(--dg-window-padding)
    var(--dg-window-padding) 0;
  overflow: hidden;
  background: var(--dg-window-bg);
}

.window-grain {
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.4;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.035'/%3E%3C/svg%3E");
}

.panel {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}
</style>
