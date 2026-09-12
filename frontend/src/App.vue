<script setup lang="ts">
import { RouterView } from 'vue-router'

import ErrorBoundary from '@/components/ErrorBoundary.vue'
import FatalErrorOverlay from '@/components/FatalErrorOverlay.vue'
import AppShell from '@/layout/AppShell.vue'
</script>

<template>
  <AppShell>
    <!--
      No page <Transition> here, deliberately. A `mode="out-in"` transition
      around lazily-loaded routes wedges: an interrupted leave can drop the
      incoming view, leaving <main> empty — a blank content area that persists
      across further navigation. Correctness beats a 200ms fade. The per-route
      key gives each route a clean instance; ErrorBoundary keeps a thrown page
      from tearing down the shell.
    -->
    <RouterView v-slot="{ Component, route }">
      <ErrorBoundary :key="route.path">
        <component :is="Component" />
      </ErrorBoundary>
    </RouterView>
  </AppShell>
  <FatalErrorOverlay />
</template>
