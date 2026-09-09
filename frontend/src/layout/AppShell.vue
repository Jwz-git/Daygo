<script setup lang="ts">
import SideRail from './SideRail.vue'
</script>

<template>
  <div class="shell">
    <!--
      A cheap grain layer that keeps large flat gradients from banding. It is
      independent of the palette, so it needs no dark-mode variant.
    -->
    <div class="window-grain" aria-hidden="true"></div>
    <div class="window-glow window-glow--cool" aria-hidden="true"></div>
    <div class="window-glow window-glow--warm" aria-hidden="true"></div>

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
  background: var(--dg-window-bg);
}

.window-grain {
  position: absolute;
  z-index: 1;
  inset: 0;
  pointer-events: none;
  opacity: 0.28;
  mix-blend-mode: soft-light;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.035'/%3E%3C/svg%3E");
}

.window-glow {
  position: absolute;
  z-index: 0;
  width: 42vw;
  height: 42vw;
  border-radius: 50%;
  pointer-events: none;
  opacity: 0.22;
  filter: blur(60px);
}

.window-glow--cool {
  top: -28vw;
  left: 4vw;
  background: #91b9e7;
}

.window-glow--warm {
  right: -18vw;
  bottom: -28vw;
  background: #efa57d;
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
