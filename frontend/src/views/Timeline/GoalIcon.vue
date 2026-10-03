<script setup lang="ts">
import { useId } from 'vue'

/*
 * The day-goal glyphs, filled in currentColor with cut-outs (via a mask, so
 * they read on any background): focus is a bullseye with an arrow struck
 * into it, distraction a gamepad. Used in the goal panels' headers and the
 * progress bubbles.
 */
defineProps<{ kind: 'focus' | 'distraction'; size?: number }>()

const maskId = `goal-icon-${useId()}`
</script>

<template>
  <svg
    class="goal-icon"
    viewBox="0 0 24 24"
    :width="size ?? 16"
    :height="size ?? 16"
    aria-hidden="true"
  >
    <template v-if="kind === 'focus'">
      <defs>
        <mask :id="maskId" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24">
          <circle cx="10.5" cy="13.5" r="9" fill="#fff" />
          <circle cx="10.5" cy="13.5" r="6.7" fill="#000" />
          <circle cx="10.5" cy="13.5" r="4.6" fill="#fff" />
          <circle cx="10.5" cy="13.5" r="2.5" fill="#000" />
          <!-- Clearance around the arrow so it lifts off the rings. -->
          <path d="M10.5 13.5 19.4 4.6" stroke="#000" stroke-width="4" stroke-linecap="round" />
        </mask>
      </defs>
      <rect width="24" height="24" fill="currentColor" :mask="`url(#${maskId})`" />
      <circle cx="10.5" cy="13.5" r="1.25" fill="currentColor" />
      <path
        d="M10.9 13.1 19.4 4.6M19.4 4.6V1.9M19.4 4.6h2.7M17.6 6.4V3.9M17.6 6.4h2.5"
        fill="none"
        stroke="currentColor"
        stroke-width="1.6"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>
    <template v-else>
      <defs>
        <mask :id="maskId" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24">
          <path
            fill="#fff"
            d="M7.3 6h9.4c2.4 0 4.5 1.7 5 4.1l1.2 5.8c.5 2.2-1.2 4.3-3.5 4.3-1.2 0-2.3-.6-3-1.6l-1.2-1.8H8.8l-1.2 1.8c-.7 1-1.8 1.6-3 1.6-2.3 0-4-2.1-3.5-4.3l1.2-5.8C2.8 7.7 4.9 6 7.3 6Z"
          />
          <!-- D-pad -->
          <rect x="6.1" y="9.3" width="1.8" height="5.6" rx="0.6" fill="#000" />
          <rect x="4.2" y="11.2" width="5.6" height="1.8" rx="0.6" fill="#000" />
          <!-- Face buttons -->
          <circle cx="16.4" cy="10.6" r="1.15" fill="#000" />
          <circle cx="18.5" cy="12.7" r="1.15" fill="#000" />
          <circle cx="14.3" cy="12.7" r="0.85" fill="#000" opacity="0.55" />
        </mask>
      </defs>
      <rect width="24" height="24" fill="currentColor" :mask="`url(#${maskId})`" />
    </template>
  </svg>
</template>

<style scoped>
.goal-icon {
  display: block;
  flex: none;
}
</style>
