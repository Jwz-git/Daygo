<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import {
  WindowHide,
  WindowIsMaximised,
  WindowMinimise,
  WindowToggleMaximise,
} from '../../wailsjs/runtime/runtime'
import appIconUrl from '../../../build/appicon.png'

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
        <span class="window-titlebar__minimise" aria-hidden="true" />
      </button>
      <button
        class="window-titlebar__control"
        type="button"
        :aria-label="$t(maximised ? 'common.window.restore' : 'common.window.maximise')"
        @click.stop="toggleMaximise"
      >
        <span
          :class="maximised ? 'window-titlebar__restore' : 'window-titlebar__maximise'"
          aria-hidden="true"
        />
      </button>
      <button
        class="window-titlebar__control window-titlebar__control--close"
        type="button"
        :aria-label="$t('common.window.close')"
        @click.stop="WindowHide"
      >
        <span class="window-titlebar__close" aria-hidden="true" />
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

.window-titlebar__minimise {
  width: 10px;
  height: 1px;
  background: currentcolor;
}

.window-titlebar__maximise {
  width: 10px;
  height: 10px;
  border: 1px solid currentcolor;
}

.window-titlebar__restore {
  position: relative;
  width: 9px;
  height: 9px;
  border: 1px solid currentcolor;
}

.window-titlebar__restore::before {
  position: absolute;
  top: -4px;
  left: 2px;
  width: 8px;
  height: 8px;
  border: 1px solid currentcolor;
  content: '';
}

.window-titlebar__restore::after {
  position: absolute;
  inset: -1px auto auto -1px;
  width: 8px;
  height: 3px;
  background: var(--dg-window-bg);
  content: '';
}

.window-titlebar__close {
  position: relative;
  width: 12px;
  height: 12px;
}

.window-titlebar__close::before,
.window-titlebar__close::after {
  position: absolute;
  top: 5.5px;
  left: 0;
  width: 12px;
  height: 1px;
  background: currentcolor;
  content: '';
  transform: rotate(45deg);
}

.window-titlebar__close::after {
  transform: rotate(-45deg);
}
</style>
