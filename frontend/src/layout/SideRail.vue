<script setup lang="ts">
import type { Component } from 'vue'
import { useRoute, type RouteLocationRaw } from 'vue-router'

import IconCaptureTest from '@/components/icons/IconCaptureTest.vue'
import IconChat from '@/components/icons/IconChat.vue'
import IconDaily from '@/components/icons/IconDaily.vue'
import IconSettings from '@/components/icons/IconSettings.vue'
import IconTimeline from '@/components/icons/IconTimeline.vue'
import IconWeekly from '@/components/icons/IconWeekly.vue'
import { calendarDayQuery } from '@/lib/calendarDate'

import SideRailItem from './SideRailItem.vue'

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
const items: readonly RailItem[] = [
  { navKey: 'timeline', to: { name: 'timeline' }, labelKey: 'nav.timeline', icon: IconTimeline },
  { navKey: 'daily', to: { name: 'daily' }, labelKey: 'nav.daily', icon: IconDaily },
  { navKey: 'weekly', to: { name: 'weekly' }, labelKey: 'nav.weekly', icon: IconWeekly },
  { navKey: 'chat', to: { name: 'chat' }, labelKey: 'nav.chat', icon: IconChat },
  { navKey: 'settings', to: { name: 'settings' }, labelKey: 'nav.settings', icon: IconSettings },
  { navKey: 'capture-test', to: { name: 'capture-test' }, labelKey: 'nav.captureTest', icon: IconCaptureTest },
]

const route = useRoute()

function destination(item: RailItem): RouteLocationRaw {
  if (item.navKey !== 'timeline' && item.navKey !== 'daily') return item.to
  const day = calendarDayQuery(route.query.day)
  return day === '' ? item.to : { name: item.navKey, query: { day } }
}
</script>

<template>
  <nav class="rail" :aria-label="$t('nav.label')">
    <ul class="rail__list">
      <li v-for="item in items" :key="item.navKey">
        <SideRailItem
          :to="destination(item)"
          :label="$t(item.labelKey)"
          :icon="item.icon"
          :active="route.meta.navKey === item.navKey"
        />
      </li>
    </ul>
  </nav>
</template>

<style scoped>
.rail {
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: var(--dg-rail-width);
  padding: 48px 0 24px;
  -webkit-app-region: drag;
  --wails-draggable: drag;
}

.rail__list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}
</style>
