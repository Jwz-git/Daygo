<script setup lang="ts">
import { RouterView } from 'vue-router'

import AppShell from '@/layout/AppShell.vue'
</script>

<template>
  <AppShell>
    <RouterView v-slot="{ Component }">
      <Transition name="page" mode="out-in">
        <component :is="Component" />
      </Transition>
    </RouterView>
  </AppShell>
</template>

<style>
/* Deliberately restrained: the legacy matchedGeometryEffect animations are not
   part of a functional-equivalence v1 (docs/09-roadmap.md). */
.page-enter-active,
.page-leave-active {
  transition: opacity 120ms ease;
}

.page-enter-from,
.page-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .page-enter-active,
  .page-leave-active {
    transition: none;
  }
}
</style>
