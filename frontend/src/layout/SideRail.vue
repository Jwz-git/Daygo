<script setup lang="ts">
import type { Component } from 'vue'
import { useRoute, type RouteLocationRaw } from 'vue-router'

import IconDaily from '@/components/icons/IconDaily.vue'
import IconSettings from '@/components/icons/IconSettings.vue'
import IconTimeline from '@/components/icons/IconTimeline.vue'
import IconWeekly from '@/components/icons/IconWeekly.vue'

import SideRailItem from './SideRailItem.vue'

interface RailItem {
  readonly navKey: string
  readonly to: RouteLocationRaw
  readonly labelKey: string
  readonly icon: Component
}

/*
 * v1 ships exactly these four destinations. A rail entry that leads nowhere is
 * worse than a missing one, so adding a page means adding one entry here — not
 * the other way round.
 */
const items: readonly RailItem[] = [
  { navKey: 'timeline', to: { name: 'timeline' }, labelKey: 'nav.timeline', icon: IconTimeline },
  { navKey: 'daily', to: { name: 'daily' }, labelKey: 'nav.daily', icon: IconDaily },
  { navKey: 'weekly', to: { name: 'weekly' }, labelKey: 'nav.weekly', icon: IconWeekly },
  { navKey: 'settings', to: { name: 'settings' }, labelKey: 'nav.settings', icon: IconSettings },
]

const route = useRoute()
</script>

<template>
  <nav class="rail" :aria-label="$t('nav.label')">
    <ul class="rail__list">
      <li v-for="item in items" :key="item.navKey">
        <SideRailItem
          :to="item.to"
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
  display: flex;
  flex-direction: column;
  align-items: center;
  width: var(--dg-rail-width);
  padding-top: 10px;
}

.rail__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}
</style>
