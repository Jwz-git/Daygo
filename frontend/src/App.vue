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
.page-enter-active {
  transition: opacity var(--dg-motion-base) ease;
}

.page-leave-active {
  transition: opacity 90ms ease;
}

.page-enter-from {
  opacity: 0;
}

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
