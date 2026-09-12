<script setup lang="ts">
import type { Component } from 'vue'
import { useRoute, type RouteLocationRaw } from 'vue-router'

import IconChat from '@/components/icons/IconChat.vue'
import IconDaily from '@/components/icons/IconDaily.vue'
import IconSettings from '@/components/icons/IconSettings.vue'
import IconTimeline from '@/components/icons/IconTimeline.vue'
import IconWeekly from '@/components/icons/IconWeekly.vue'
import { calendarDayQuery } from '@/lib/calendarDate'

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
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
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
