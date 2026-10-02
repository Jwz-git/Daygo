<script setup lang="ts">
import { computed } from 'vue'

import { glyphs, viewBoxStroke, type GlyphPath, type IconName } from './icons/glyphs'

/*
 * The one way to draw an interface icon. `size` is the rendered size in CSS
 * pixels; it also picks the stroke weight (see glyphs.ts), so pass the size
 * the icon actually renders at even when a stylesheet sets its box.
 *
 * Icons are decorative: the control that holds one carries the accessible
 * name (aria-label or visible text), so the SVG is hidden from assistive tech.
 */
const props = withDefaults(defineProps<{ name: IconName; size?: number }>(), { size: 16 })

const paths = computed<readonly GlyphPath[]>(() => glyphs[props.name])
const stroke = computed(() => viewBoxStroke(props.size))
</script>

<template>
  <svg
    class="dg-icon"
    :width="props.size"
    :height="props.size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="stroke"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path
      v-for="(path, index) in paths"
      :key="index"
      :d="path.d"
      :fill="path.filled ? 'currentColor' : undefined"
      :stroke="path.filled ? 'none' : undefined"
    />
  </svg>
</template>

<style scoped>
.dg-icon {
  display: inline-block;
  flex: none;
  vertical-align: middle;
  /* Keep the stroke on the pixel grid's intent rather than WebKit's snap,
     which jitters thin strokes as the icon enters its own layer. */
  shape-rendering: geometricPrecision;
}
</style>
