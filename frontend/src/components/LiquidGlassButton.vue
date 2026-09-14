<script setup lang="ts">
/*
 * Liquid-glass button with three variants.
 *
 *   - primary  — accent fill, dense visual weight (used sparingly for the
 *                primary action on a screen)
 *   - secondary — neutral glass surface, the default for in-card actions
 *   - ghost    — transparent until hovered; for tertiary/icon buttons
 *
 * Hover applies a small tint lift via CSS only. Pointer tracking is
 * delegated to the parent surface.
 */
import { computed } from 'vue'

type Variant = 'primary' | 'secondary' | 'ghost'

const props = withDefaults(
  defineProps<{
    variant?: Variant
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
    tracking?: boolean
  }>(),
  {
    variant: 'secondary',
    type: 'button',
    disabled: false,
    tracking: false,
  },
)

const emit = defineEmits<{
  click: [event: MouseEvent]
}>()

const classes = computed(() => [
  'lg-button',
  `lg-button--${props.variant}`,
  'lg-surface',
  'lg-air',
  { 'lg-tracking': props.tracking, 'is-disabled': props.disabled },
])

function handleClick(event: MouseEvent): void {
  if (props.disabled) return
  emit('click', event)
}
</script>

<template>
  <button
    :type="props.type"
    :class="classes"
    :disabled="props.disabled"
    :aria-disabled="props.disabled || undefined"
    @click="handleClick"
  >
    <slot />
  </button>
</template>

<style scoped>
.lg-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 32px;
  padding: 6px 14px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 550;
  white-space: nowrap;
  cursor: pointer;
  user-select: none;
  /* Reset native button surfaces — the .lg-surface class supplies the
     glass. Without this the Safari desktop WebView would render Aqua. */
  appearance: none;
  -webkit-appearance: none;
  background-clip: padding-box;
  transition:
    background var(--dg-motion-fast) ease,
    border-color var(--dg-motion-fast) ease,
    box-shadow var(--dg-motion-base) ease,
    transform var(--dg-motion-base) var(--dg-ease-glide);
}

/* ─── secondary (default) ─────────────────────────────────────────────── */
.lg-button--secondary {
  color: var(--dg-text-primary);
  /* Tint bump on hover gives the click a glass-lift cue without changing
     the geometry. */
}
.lg-button--secondary:not(.is-disabled):hover {
  background: var(--lg-tint-glass);
  border-color: var(--lg-border-glass);
}

/* ─── primary ─────────────────────────────────────────────────────────── */
.lg-button--primary {
  background: var(--dg-button-primary-fill);
  border-color: var(--dg-button-primary-border);
  color: var(--dg-button-primary-text);
  box-shadow: var(--dg-button-primary-shadow);
}
.lg-button--primary:not(.is-disabled):hover {
  background: var(--dg-button-primary-hover);
}
.lg-button--primary::before {
  /* Override the default directional sheen with a subtle gradient for the
     primary surface; still respects the upper-left light source. */
  background: linear-gradient(
    135deg,
    rgb(255 255 255 / 0.28),
    transparent 50%
  );
}

/* ─── ghost ───────────────────────────────────────────────────────────── */
.lg-button--ghost {
  background: transparent;
  border-color: transparent;
  box-shadow: none;
  color: var(--dg-text-secondary);
  -webkit-backdrop-filter: none;
  backdrop-filter: none;
}
.lg-button--ghost:not(.is-disabled):hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
}

/* ─── disabled ────────────────────────────────────────────────────────── */
.lg-button.is-disabled,
.lg-button:disabled {
  opacity: 0.48;
  cursor: default;
  box-shadow: none;
}

/* ─── press ───────────────────────────────────────────────────────────── */
.lg-button:not(.is-disabled):active {
  transform: scale(0.97);
  box-shadow: inset 0 1px 2px rgb(15 23 42 / 0.12);
  transition-duration: var(--dg-motion-fast);
}

.lg-button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

@media (prefers-reduced-motion: reduce) {
  .lg-button {
    transition: none;
  }
  .lg-button:not(.is-disabled):active {
    transform: none;
  }
}

@media (forced-colors: active) {
  .lg-button {
    background: ButtonFace;
    color: ButtonText;
    border: 1px solid ButtonText;
    box-shadow: none;
  }
}
</style>
