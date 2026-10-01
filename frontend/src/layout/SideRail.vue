<script setup lang="ts">
import type { Component } from 'vue'
import { computed } from 'vue'
import { useRoute, type RouteLocationRaw } from 'vue-router'

import IconCaptureTest from '@/components/icons/IconCaptureTest.vue'
import IconChat from '@/components/icons/IconChat.vue'
import IconDaily from '@/components/icons/IconDaily.vue'
import IconSettings from '@/components/icons/IconSettings.vue'
import IconTimeline from '@/components/icons/IconTimeline.vue'
import IconWeekly from '@/components/icons/IconWeekly.vue'
import { calendarDayQuery } from '@/lib/calendarDate'
import { useTestToolsStore } from '@/stores/testTools'

import SideRailItem from './SideRailItem.vue'
import RecordingControl from './RecordingControl.vue'

interface RailItem {
  readonly navKey: string
  readonly to: RouteLocationRaw
  readonly labelKey: string
  readonly icon: Component
}

/*
 * A rail entry exists only when its page exists (a placeholder page counts;
 * a dead link does not). Adding a page means adding one entry here — not the
 * other way round.
 */
const mainItems: readonly RailItem[] = [
  { navKey: 'timeline', to: { name: 'timeline' }, labelKey: 'nav.timeline', icon: IconTimeline },
  { navKey: 'daily', to: { name: 'daily' }, labelKey: 'nav.daily', icon: IconDaily },
  { navKey: 'weekly', to: { name: 'weekly' }, labelKey: 'nav.weekly', icon: IconWeekly },
  { navKey: 'chat', to: { name: 'chat' }, labelKey: 'nav.chat', icon: IconChat },
]
const utilityItems: readonly RailItem[] = [
  { navKey: 'settings', to: { name: 'settings' }, labelKey: 'nav.settings', icon: IconSettings },
]

const route = useRoute()
const testTools = useTestToolsStore()

// The test page ships only in dev/opt-in builds; the production installer
// tree-shakes it out, so the rail entry must be gone even if the setting is on.
const testToolsVisible = computed(() => __DAYGO_TEST_TOOLS__ && testTools.enabled)

function destination(item: RailItem): RouteLocationRaw {
  if (item.navKey !== 'timeline' && item.navKey !== 'daily') return item.to
  const day = calendarDayQuery(route.query.day)
  return day === '' ? item.to : { name: item.navKey, query: { day } }
}
</script>

<template>
  <nav class="rail" :aria-label="$t('nav.label')">
    <ul class="rail__list">
      <li v-for="item in mainItems" :key="item.navKey">
        <SideRailItem
          :to="destination(item)"
          :label="$t(item.labelKey)"
          :icon="item.icon"
          :active="route.meta.navKey === item.navKey"
        />
      </li>
    </ul>
    <div class="rail__utility">
      <RecordingControl />
      <SideRailItem
        v-if="testToolsVisible"
        :to="{ name: 'test' }"
        :label="$t('nav.test')"
        :icon="IconCaptureTest"
        :active="route.meta.navKey === 'test'"
      />
      <SideRailItem
        v-for="item in utilityItems"
        :key="item.navKey"
        :to="destination(item)"
        :label="$t(item.labelKey)"
        :icon="item.icon"
        :active="route.meta.navKey === item.navKey"
      />
    </div>
  </nav>
</template>

<style scoped>
.rail {
  position: relative;
  /* Above .panel so the recording popover can overlay the main content:
     the popover's own z-index is trapped inside this stacking context. */
  z-index: 3;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  width: var(--dg-rail-width);
  margin: var(--dg-window-padding) 0 var(--dg-window-padding) var(--dg-window-padding);
  /* Clears the traffic lights, which sit inside the floating pane on macOS. */
  padding: 46px 0 12px;
  border: 0.5px solid var(--dg-sidebar-border);
  border-radius: var(--dg-sidebar-radius);
  background: var(--dg-glass-fallback);
  box-shadow: var(--dg-glass-shadow);
  -webkit-app-region: drag;
  --wails-draggable: drag;
}

@supports ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
  .rail {
    background: var(--dg-sidebar-fill);
    -webkit-backdrop-filter: blur(var(--dg-glass-blur)) saturate(var(--dg-glass-saturation));
    backdrop-filter: blur(var(--dg-glass-blur)) saturate(var(--dg-glass-saturation));
  }
}

/* Specular top light: the one cue that the pane is a material, not a box. */
.rail::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  border-radius: inherit;
  background: var(--dg-glass-sheen);
  pointer-events: none;
}

/* Windows has its own title bar row; nothing to clear at the top. */
:root[data-dg-platform='windows'] .rail {
  padding-top: 14px;
}

@media (forced-colors: active) {
  .rail {
    border: 1px solid CanvasText;
    background: Canvas;
  }

  .rail::before {
    display: none;
  }
}

.rail__list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}

.rail__utility {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
  margin-top: auto;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}
</style>
