<script setup lang="ts">
import { onErrorCaptured, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { reportFatalError } from '@/lib/fatalError'

/**
 * Contains a render or lifecycle error thrown by the routed page so it cannot
 * tear down the shell or wedge the page <Transition> — either of which blanks
 * every route, not just the one that failed.
 *
 * The readable details live in the top-level FatalErrorOverlay; this boundary
 * only stops the crash and then renders nothing in the failed subtree, so the
 * broken component is not re-invoked into a throw loop.
 */
const failed = ref(false)
const route = useRoute()

onErrorCaptured((error, _instance, info) => {
  failed.value = true
  reportFatalError(`render (${info})`, error)
  // Handled here; do not propagate to app.config.errorHandler as well.
  return false
})

// A new location is a clean slate — retry rendering the page. App.vue keys this
// boundary by route.path, so cross-route navigation already remounts it; this
// watch only covers same-path changes (e.g. the timeline ?day= query).
watch(
  () => route.fullPath,
  () => {
    failed.value = false
  },
)
</script>

<template>
  <slot v-if="!failed" />
  <div v-else class="error-boundary" aria-hidden="true"></div>
</template>

<style scoped>
.error-boundary {
  min-height: 0;
}
</style>
