<script setup lang="ts">
/*
 * Liquid-glass chip — replaces the previous .dg-chip class.
 *
 * Two roles:
 *   - filled    — selected/active state; stronger tint, primary text
 *   - outlined  — default state; subtle tint, secondary text
 *
 * `interactive` enables hover/active affordances for click chips; default
 * `false` keeps the chip presentational so it can be used inside nav
 * lists without spurious hover hints.
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    filled?: boolean
    interactive?: boolean
    disabled?: boolean
  }>(),
  {
    filled: false,
    interactive: false,
    disabled: false,
  },
)

const classes = computed(() => [
  'lg-chip',
  'lg-surface',
  'lg-air',
  { 'lg-chip--filled': props.filled, 'lg-chip--interactive': props.interactive, 'is-disabled': props.disabled },
])
</script>

<template>
  <span :class="classes" :aria-disabled="props.disabled || undefined">
    <slot />
  </span>
</template>

<style scoped>
.lg-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  color: var(--dg-text-secondary);
  user-select: none;
  transition:
    background var(--dg-motion-fast) ease,
    color var(--dg-motion-fast) ease,
    transform var(--dg-motion-base) var(--dg-ease-glide);
}

.lg-chip--filled {
  color: var(--dg-text-primary);
  background: var(--dg-control-fill);
  border-color: transparent;
}

.lg-chip--interactive {
  cursor: pointer;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}

.lg-chip--interactive:not(.is-disabled):hover {
  background: var(--dg-control-fill-hover);
  color: var(--dg-text-primary);
}

.lg-chip--interactive:not(.is-disabled):active {
  transform: scale(0.96);
  transition-duration: var(--dg-motion-fast);
}

.lg-chip.is-disabled,
.lg-chip[aria-disabled='true'] {
  opacity: 0.48;
  cursor: default;
}

.lg-chip:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

@media (prefers-reduced-motion: reduce) {
  .lg-chip {
    transition: none;
  }
  .lg-chip--interactive:not(.is-disabled):active {
    transform: none;
  }
}

@media (forced-colors: active) {
  .lg-chip {
    background: ButtonFace;
    color: ButtonText;
    border: 1px solid ButtonText;
  }
  .lg-chip--filled {
    background: Highlight;
    color: HighlightText;
  }
}
</style>
