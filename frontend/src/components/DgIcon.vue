<script setup lang="ts">
import { computed, useId } from 'vue'

import { glyphs, viewBoxStroke, type Glyph, type IconName } from './icons/glyphs'

/*
 * The one way to draw an interface icon. `size` is the rendered size in CSS
 * pixels; it also picks the stroke weight of line glyphs (see glyphs.ts), so
 * pass the size the icon actually renders at even when a stylesheet sets its
 * box.
 *
 * Asset glyphs (Dayflow's own artwork) are drawn as an alpha mask filled with
 * currentColor, the web equivalent of SwiftUI's template rendering: the file
 * supplies the shape, the surrounding text colour supplies the tint. The root
 * stays an <svg> either way, so callers' `svg` sizing rules keep applying.
 *
 * Icons are decorative: the control that holds one carries the accessible
 * name (aria-label or visible text), so the SVG is hidden from assistive tech.
 */
const props = withDefaults(defineProps<{ name: IconName; size?: number }>(), { size: 16 })

// Unique per instance and stable across renders; the mask is referenced by id.
const maskId = `dg-icon-mask-${useId()}`

const glyph = computed<Glyph>(() => glyphs[props.name])
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
    <template v-if="glyph.kind === 'asset'">
      <defs>
        <mask :id="maskId" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24" style="mask-type: alpha">
          <image
            :href="glyph.src"
            x="0"
            y="0"
            width="24"
            height="24"
            preserveAspectRatio="xMidYMid meet"
            :transform="glyph.flipY ? 'translate(0 24) scale(1 -1)' : undefined"
          />
        </mask>
      </defs>
      <rect width="24" height="24" fill="currentColor" stroke="none" :mask="`url(#${maskId})`" />
    </template>
    <template v-else>
      <path
        v-for="(path, index) in glyph.paths"
        :key="index"
        :d="path.d"
        :fill="path.filled ? 'currentColor' : undefined"
        :fill-rule="path.filled ? 'evenodd' : undefined"
        :stroke="path.filled ? 'none' : undefined"
      />
    </template>
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
