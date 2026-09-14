<script setup lang="ts">
/*
 * Generic liquid-glass surface.
 *
 * Three intensity levels:
 *   - air   — chips, small icon buttons (blur 12-18 px, tint ~10%)
 *   - glass — navigation, cards, dropdowns (blur 18-28 px, tint ~14%)
 *   - dense — modal, mobile drawer, key menus (blur 24-36 px, tint ~22%)
 *
 * `tracking` enables a pointer-following specular highlight via
 * mousemove → --lg-mouse-x / --lg-mouse-y on the element. The tracking
 * logic is opt-in because the global mousemove listener runs in AppShell
 * and is responsible for fewer listeners overall.
 */
import { computed } from 'vue'

type Intensity = 'air' | 'glass' | 'dense'

const props = withDefaults(
  defineProps<{
    intensity?: Intensity
    tracking?: boolean
    as?: keyof HTMLElementTagNameMap
  }>(),
  {
    intensity: 'glass',
    tracking: false,
    as: 'div',
  },
)

const classes = computed(() => [
  'lg-surface',
  `lg-${props.intensity}`,
  { 'lg-tracking': props.tracking },
])
</script>

<template>
  <component :is="props.as" :class="classes">
    <slot />
  </component>
</template>

<style scoped>
/* Pointer-tracking is wired by CSS custom properties; the
   .lg-tracking class only toggles visibility/opacity of the ::after
   highlight. Coordinates are written by AppShell's pointer listener. */
</style>
