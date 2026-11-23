<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import {
  hideDesktopWindow as WindowHide,
  desktopIsMaximised as WindowIsMaximised,
  minimiseDesktopWindow as WindowMinimise,
  toggleDesktopMaximise as WindowToggleMaximise,
} from '@/api/desktopBridge'
import appIconUrl from '@/assets/window-titlebar-icon.png'

const maximised = ref(false)

async function refreshMaximised(): Promise<void> {
  maximised.value = await WindowIsMaximised()
}

async function toggleMaximise(): Promise<void> {
  WindowToggleMaximise()
  // Wails updates the native state asynchronously after the command returns.
  window.requestAnimationFrame(() => void refreshMaximised())
}

function onResize(): void {
  void refreshMaximised()
}

onMounted(() => {
  void refreshMaximised()
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => window.removeEventListener('resize', onResize))
</script>

<template>
  <header class="window-titlebar" @dblclick="toggleMaximise">
    <img class="window-titlebar__icon" :src="appIconUrl" alt="" draggable="false" />
    <div class="window-titlebar__controls" @dblclick.stop>
      <button
        class="window-titlebar__control"
        type="button"
        :aria-label="$t('common.window.minimise')"
        @click.stop="WindowMinimise"
      >
        <svg class="window-titlebar__glyph" viewBox="0 0 10 10" width="10" height="10" aria-hidden="true">
          <path d="M0 5h10" />
        </svg>
      </button>
      <button
        class="window-titlebar__control"
        type="button"
        :aria-label="$t(maximised ? 'common.window.restore' : 'common.window.maximise')"
        @click.stop="toggleMaximise"
      >
        <svg v-if="maximised" class="window-titlebar__glyph" viewBox="0 0 10 10" width="10" height="10" aria-hidden="true">
          <path d="M2.5 2.5V0.5h7v7h-2" />
          <rect x="0.5" y="2.5" width="7" height="7" />
        </svg>
        <svg v-else class="window-titlebar__glyph" viewBox="0 0 10 10" width="10" height="10" aria-hidden="true">
          <rect x="0.5" y="0.5" width="9" height="9" />
        </svg>
      </button>
      <button
        class="window-titlebar__control window-titlebar__control--close"
        type="button"
        :aria-label="$t('common.window.close')"
        @click.stop="WindowHide"
      >
        <svg class="window-titlebar__glyph" viewBox="0 0 10 10" width="10" height="10" aria-hidden="true">
          <path d="M1 1l8 8M9 1 1 9" />
        </svg>
      </button>
    </div>
  </header>
</template>

<style scoped>
.window-titlebar {
  position: relative;
  z-index: 5;
  display: flex;
  flex: 0 0 var(--dg-windows-titlebar-height);
  align-items: center;
  justify-content: space-between;
  height: var(--dg-windows-titlebar-height);
  border-bottom: 1px solid var(--dg-titlebar-border);
  user-select: none;
  -webkit-app-region: drag;
  --wails-draggable: drag;
}

.window-titlebar__icon {
  width: 18px;
  height: 18px;
  margin-left: 12px;
  border-radius: 4px;
  pointer-events: none;
}

.window-titlebar__controls {
  display: flex;
  align-self: stretch;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}

.window-titlebar__control {
  position: relative;
  display: grid;
  width: 46px;
  height: 100%;
  padding: 0;
  border: 0;
  border-radius: 0;
  color: var(--dg-text-secondary);
  background: transparent;
  transition: color var(--dg-motion-fast) ease, background var(--dg-motion-fast) ease;
  place-items: center;
}

.window-titlebar__control:hover {
  color: var(--dg-text-primary);
  background: var(--dg-hover-fill-strong);
}

.window-titlebar__control--close:hover {
  color: #fff;
  background: #c42b1c;
}

.window-titlebar__glyph {
  fill: none;
  stroke: currentcolor;
  stroke-width: 1;
  shape-rendering: crispEdges;
}

.window-titlebar__control--close .window-titlebar__glyph {
  shape-rendering: geometricPrecision;
}
</style>
